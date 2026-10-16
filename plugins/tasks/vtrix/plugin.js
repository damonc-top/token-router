// Vendor caps from the Vtrix API documents.
const MAX_IMAGE_INPUTS_EDIT = 4;
const MAX_IMAGE_INPUTS_INTERLEAVE = 1;
const MAX_IMAGE_OUTPUTS = 16;
const MAX_IMAGE_BYTES = 10485760; // 10 MB input image limit
const MAX_IMAGE_PROMPT_RUNES = 2000;
const MAX_NEGATIVE_PROMPT_RUNES = 500;
const MAX_THREED_PROMPT_RUNES = 1024;
const MIN_FACE_COUNT = 3000;
const MAX_FACE_COUNT = 1500000;
// Mirrors the host task duration bound (relaycommon.MaxTaskDurationSeconds).
const MAX_DURATION_SECONDS = 3600;

const IMAGE_UNIT_LABEL = { en: "image", zh: "张", "zh-TW": "張" };
const ASSET_UNIT_LABEL = { en: "asset", zh: "件", "zh-TW": "件" };

const IMAGE_USAGE_SCHEMA = {
  image_count: {
    type: "number",
    unit: "count",
    unitLabel: IMAGE_UNIT_LABEL,
    description: { en: "Image generation unit price", zh: "图片生成单价" },
  },
};

const VIDEO_USAGE_SCHEMA = {
  duration: {
    type: "number",
    unit: "second",
    description: { en: "Video generation unit price", zh: "视频生成单价" },
  },
  resolution: {
    enum: ["480P", "720P", "1080P"],
    enumLabels: {
      "480P": { en: "480P", zh: "480P" },
      "720P": { en: "720P", zh: "720P" },
      "1080P": { en: "1080P", zh: "1080P" },
    },
    description: { en: "Output video resolution", zh: "输出视频分辨率" },
  },
};

const THREED_USAGE_SCHEMA = {
  asset_count: {
    type: "number",
    unit: "count",
    unitLabel: ASSET_UNIT_LABEL,
    description: { en: "3D asset generation unit price", zh: "3D 资产生成单价" },
  },
};

const IMAGE_USAGE_EXAMPLES = [
  { label: "edit · n2", facts: { image_count: 2 } },
  { label: "interleave · 5", facts: { image_count: 5 } },
];
const VIDEO_USAGE_EXAMPLES = [
  { label: "i2v · 5s 720P", facts: { duration: 5, resolution: "720P" } },
  { label: "i2v · 10s 1080P", facts: { duration: 10, resolution: "1080P" } },
];
const THREED_USAGE_EXAMPLES = [{ label: "standard", facts: { asset_count: 1 } }];

const VIDEO_RESOLUTIONS = ["480P", "720P", "1080P"];
const THREED_GENERATE_TYPES = ["Normal", "LowPoly", "Geometry", "Sketch"];
const THREED_POLYGON_TYPES = ["triangle", "quadrilateral"];
const THREED_VIEW_TYPES = ["left", "right", "back", "top", "bottom", "left_front", "right_front"];
const VIDEO_MEDIA_TYPES = ["first_frame", "last_frame", "driving_audio", "first_clip"];

export const meta = {
  apiVersion: 1,
  key: "vtrix",
  name: "Vtrix",
  description: {
    en: "Vtrix image, video, and 3D generation via the unified generation API",
    zh: "Vtrix 统一生成接口的图片、视频与 3D 生成",
  },
  version: "1.0.0",
  author: { name: "afeng" },
  baseUrl: "https://cloud.vtrix.ai",
  models: ["wan26_image", "wan27_i2v", "tencent_hunyuan_3d_pro"],
  fetchMode: "per_task",
  usageSchema: IMAGE_USAGE_SCHEMA,
  usageProfiles: [
    { models: ["wan27_i2v"], schema: VIDEO_USAGE_SCHEMA, examples: VIDEO_USAGE_EXAMPLES },
    { models: ["tencent_hunyuan_3d_pro"], schema: THREED_USAGE_SCHEMA, examples: THREED_USAGE_EXAMPLES },
  ],
  usageExamples: IMAGE_USAGE_EXAMPLES,
  routes: [
    {
      method: "POST",
      path: "/vtrix/v1/generation",
      type: "submit",
      models: ["wan26_image", "wan27_i2v", "tencent_hunyuan_3d_pro"],
      decode: "submitGeneration",
      render: "generationCreated",
    },
    { method: "GET", path: "/vtrix/v1/generation/task/:task_id", type: "query", render: "generationStatus" },
  ],
  protocols: [
    { name: "openai_image", models: ["wan26_image"] },
    { name: "openai_video", models: ["wan27_i2v"] },
  ],
};

function trimmed(value) {
  return String(value === undefined || value === null ? "" : value).trim();
}

function isImageInput(value) {
  return /^https?:\/\//i.test(trimmed(value)) || /^data:[^;,]*;base64,/i.test(trimmed(value));
}

function filePlaceholder(ref) {
  return { __fileRef: ref, encoding: "dataUrl", maxBytes: MAX_IMAGE_BYTES };
}

function requireJsonObject(ctx) {
  if (!ctx.body || ctx.body.kind !== "json" || !ctx.body.value || typeof ctx.body.value !== "object" || Array.isArray(ctx.body.value))
    throw new Error("JSON object required");
  return ctx.body.value;
}

function optionalBool(value, name) {
  if (value === undefined || value === null) return undefined;
  if (typeof value === "boolean") return value;
  if (value === "true") return true;
  if (value === "false") return false;
  throw new Error(name + " must be true or false");
}

function optionalInt(value, name, min, max) {
  if (value === undefined || value === null || value === "") return undefined;
  const n = Number(value);
  if (!Number.isInteger(n) || n < min || n > max)
    throw new Error(name + " must be an integer between " + min + " and " + max);
  return n;
}

function normalizeImageSize(value) {
  const raw = trimmed(value);
  if (!raw || raw.toLowerCase() === "auto") return "1K";
  const upper = raw.toUpperCase();
  if (upper === "1K" || upper === "2K") return upper;
  const parts = raw.replace("*", "x").split("x");
  if (parts.length === 2) {
    const width = Number(parts[0]);
    const height = Number(parts[1]);
    if (Number.isInteger(width) && Number.isInteger(height) && width >= 768 && width <= 2048 && height >= 768 && height <= 2048)
      return raw;
  }
  throw new Error("size must be 1K, 2K, or WxH with both sides between 768 and 2048");
}

function normalizeVideoResolution(value) {
  const raw = trimmed(value).toUpperCase();
  if (!raw) return "720P";
  if (VIDEO_RESOLUTIONS.indexOf(raw) >= 0) return raw;
  throw new Error("resolution must be one of 480P, 720P, 1080P");
}

function imageCountFor(req) {
  return req.interleave ? req.max_images : req.n;
}

// Flat image request shared by the OpenAI image protocol and the native route.
// interleave=true maps to the vendor's interleaved generation (0-1 reference
// images, count via max_images); interleave=false is image editing (1-4
// reference images, count via n).
function validateImageRequest(req) {
  const prompt = trimmed(req.prompt);
  if (!prompt) throw new Error("prompt is required");
  if (prompt.length > MAX_IMAGE_PROMPT_RUNES) throw new Error("prompt must be at most " + MAX_IMAGE_PROMPT_RUNES + " characters");
  const negative = trimmed(req.negative_prompt);
  if (negative.length > MAX_NEGATIVE_PROMPT_RUNES)
    throw new Error("negative_prompt must be at most " + MAX_NEGATIVE_PROMPT_RUNES + " characters");
  const images = [];
  for (const image of req.images || []) {
    if (image && typeof image === "object" && image.__fileRef) {
      images.push(filePlaceholder(image.__fileRef));
      continue;
    }
    if (!isImageInput(image)) throw new Error("image must be an HTTP URL or a Base64 data URL");
    images.push(trimmed(image));
  }
  const flat = {
    model: req.model,
    prompt: prompt,
    images: images,
    interleave: req.interleave === true,
    size: normalizeImageSize(req.size),
  };
  if (negative) flat.negative_prompt = negative;
  if (flat.interleave) {
    if (images.length > MAX_IMAGE_INPUTS_INTERLEAVE)
      throw new Error("interleaved generation accepts at most " + MAX_IMAGE_INPUTS_INTERLEAVE + " reference image");
    if (req.n !== undefined && Number(req.n) !== 1) throw new Error("interleaved generation requires n to be 1; use max_images instead");
    flat.max_images = optionalInt(req.max_images === undefined ? 5 : req.max_images, "max_images", 1, 5);
  } else {
    if (images.length < 1) throw new Error("image is required for editing");
    if (images.length > MAX_IMAGE_INPUTS_EDIT)
      throw new Error("editing accepts at most " + MAX_IMAGE_INPUTS_EDIT + " reference images");
    flat.n = optionalInt(req.n === undefined ? 1 : req.n, "n", 1, 4);
  }
  const watermark = optionalBool(req.watermark, "watermark");
  if (watermark !== undefined) flat.watermark = watermark;
  const seed = optionalInt(req.seed, "seed", 0, 2147483647);
  if (seed !== undefined) flat.seed = seed;
  if (!flat.interleave) {
    const promptExtend = optionalBool(req.prompt_extend, "prompt_extend");
    if (promptExtend !== undefined) flat.prompt_extend = promptExtend;
  }
  return flat;
}

function imageRequestFromOpenAI(ctx) {
  let req = {};
  const uploads = [];
  if (ctx.body && ctx.body.kind === "json") {
    req = ctx.body.value;
    if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
  } else if (ctx.body && ctx.body.kind === "multipart") {
    const fields = ctx.body.fields || {};
    for (const name of Object.keys(fields)) {
      if (fields[name].length > 1) throw new Error(name + " must be provided once");
      req[name] = fields[name][0];
    }
    for (const key of ["n", "max_images", "seed"]) {
      if (req[key] !== undefined) req[key] = Number(req[key]);
    }
    for (const file of ctx.body.files || []) {
      if (/^image(\[\d*\])?$/.test(file.field)) uploads.push(file.ref);
      else throw new Error("unexpected file field: " + file.field);
    }
  } else throw new Error("JSON or multipart body required");
  if (req.response_format !== undefined && req.response_format !== "url" && req.response_format !== "b64_json")
    throw new Error("response_format must be url or b64_json");
  const images = [];
  for (const image of [].concat(req.image === undefined ? [] : req.image, req.images === undefined ? [] : req.images)) {
    if (typeof image === "string" && trimmed(image)) images.push(trimmed(image));
    else throw new Error("image must be an HTTP URL or a Base64 data URL");
  }
  for (const ref of uploads) images.push({ __fileRef: ref });
  const interleave = ctx.operation !== "edit";
  if (interleave && req.n !== undefined && (!Number.isInteger(Number(req.n)) || Number(req.n) < 1 || Number(req.n) > 5))
    throw new Error("n must be an integer between 1 and 5");
  const flat = validateImageRequest({
    model: ctx.model,
    prompt: req.prompt,
    images: images,
    n: interleave ? 1 : req.n,
    max_images: interleave ? req.n : req.max_images,
    size: req.size,
    negative_prompt: req.negative_prompt,
    watermark: req.watermark,
    seed: req.seed,
    prompt_extend: req.prompt_extend,
    interleave: interleave,
  });
  if (ctx.operation === "edit" && !uploads.length && !images.length) throw new Error("image is required");
  return flat;
}

// Native route: the client already speaks the Vtrix wire format.
function imageRequestFromVtrix(req) {
  const input = (req.input || [])[0] || {};
  const params = input.params || {};
  const messages = ((params.input || {}).messages || [])[0] || {};
  const content = Array.isArray(messages.content) ? messages.content : [];
  const images = [];
  let prompt = "";
  for (const part of content) {
    if (!part || typeof part !== "object") continue;
    if (typeof part.image === "string" && trimmed(part.image)) images.push(trimmed(part.image));
    else if (typeof part.text === "string") prompt = part.text;
  }
  const parameters = params.parameters || {};
  return validateImageRequest({
    model: req.model,
    prompt: prompt,
    images: images,
    n: parameters.n,
    max_images: parameters.max_images,
    size: parameters.size,
    negative_prompt: parameters.negative_prompt,
    watermark: parameters.watermark,
    seed: parameters.seed,
    prompt_extend: parameters.prompt_extend,
    interleave: parameters.enable_interleave === true,
  });
}

// Flat video request shared by the OpenAI video protocol and the native route.
// wan27_i2v generates from frames, so at least one first_frame media entry is
// required; the remaining vendor media kinds pass through unchanged.
function validateVideoRequest(req) {
  const prompt = trimmed(req.prompt);
  const media = [];
  for (const entry of req.media || []) {
    if (!entry || typeof entry !== "object") throw new Error("media entries must be objects");
    const type = trimmed(entry.type);
    if (VIDEO_MEDIA_TYPES.indexOf(type) < 0)
      throw new Error("media type must be one of " + VIDEO_MEDIA_TYPES.join(", "));
    let url = entry.url;
    if (url && typeof url === "object" && url.__fileRef) url = filePlaceholder(url.__fileRef);
    if (!isImageInput(url) || typeof url !== "string") {
      if (!(url && typeof url === "object" && url.__fileRef)) throw new Error("media url must be an HTTP URL or a Base64 data URL");
    }
    media.push({ type: type, url: url });
  }
  const hasFilePlaceholder = media.some(function (entry) {
    return entry.url && typeof entry.url === "object" && entry.url.__fileRef;
  });
  if (!prompt && !media.length) throw new Error("prompt or a first_frame media entry is required");
  const firstFrame = media.filter(function (entry) {
    return entry.type === "first_frame";
  });
  if (!firstFrame.length && !hasFilePlaceholder && !prompt)
    throw new Error("wan27_i2v requires a first_frame media entry");
  if (!firstFrame.length && !hasFilePlaceholder)
    throw new Error("wan27_i2v requires a first_frame media entry");
  const duration = optionalInt(req.duration === undefined ? 5 : req.duration, "duration", 1, MAX_DURATION_SECONDS);
  const flat = {
    model: req.model,
    prompt: prompt,
    media: media,
    duration: duration,
    resolution: normalizeVideoResolution(req.resolution),
  };
  const negative = trimmed(req.negative_prompt);
  if (negative) flat.negative_prompt = negative;
  const ratio = trimmed(req.ratio);
  if (ratio) {
    if (!/^\d+:\d+$/.test(ratio)) throw new Error("ratio must look like 16:9");
    flat.ratio = ratio;
  }
  const watermark = optionalBool(req.watermark, "watermark");
  if (watermark !== undefined) flat.watermark = watermark;
  const promptExtend = optionalBool(req.prompt_extend, "prompt_extend");
  if (promptExtend !== undefined) flat.prompt_extend = promptExtend;
  const seed = optionalInt(req.seed, "seed", 0, 2147483647);
  if (seed !== undefined) flat.seed = seed;
  const audioSetting = trimmed(req.audio_setting);
  if (audioSetting) {
    if (audioSetting !== "auto" && audioSetting !== "origin") throw new Error("audio_setting must be auto or origin");
    flat.audio_setting = audioSetting;
  }
  return flat;
}

function videoRequestFromOpenAI(ctx) {
  let req;
  let hasInputReferenceFile = false;
  if (ctx.body && ctx.body.kind === "json") {
    req = ctx.body.value;
    if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
  } else if (ctx.body && ctx.body.kind === "multipart") {
    const fields = ctx.body.fields || {};
    req = {};
    for (const name of Object.keys(fields)) {
      const values = fields[name];
      if (name !== "input_reference" && values.length > 1) throw new Error(name + " must be provided once");
      req[name] = values[0];
    }
    for (const file of ctx.body.files || []) {
      if (file.field !== "input_reference") throw new Error("unexpected file field: " + file.field);
      if (hasInputReferenceFile) throw new Error("input_reference must be provided once");
      hasInputReferenceFile = true;
    }
    if (req.metadata !== undefined) {
      let parsed;
      try {
        parsed = JSON.parse(req.metadata);
      } catch (e) {
        throw new Error("metadata must be a JSON object string");
      }
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error("metadata must be a JSON object string");
      req = Object.assign({}, parsed, req);
      delete req.metadata;
    }
  } else throw new Error("JSON or multipart body required");
  if (req.seconds !== undefined) req.duration = Number(req.seconds);
  if (req.size !== undefined && req.resolution === undefined) req.resolution = req.size;
  const media = [];
  if (hasInputReferenceFile) media.push({ type: "first_frame", url: { __fileRef: "request_file:input_reference" } });
  else if (trimmed(req.input_reference)) media.push({ type: "first_frame", url: trimmed(req.input_reference) });
  return validateVideoRequest({
    model: ctx.model,
    prompt: req.prompt,
    negative_prompt: req.negative_prompt,
    media: media,
    duration: req.duration,
    resolution: req.resolution,
    ratio: req.ratio,
    watermark: req.watermark,
    seed: req.seed,
    prompt_extend: req.prompt_extend,
    audio_setting: req.audio_setting,
  });
}

function videoRequestFromVtrix(req) {
  const input = (req.input || [])[0] || {};
  const params = input.params || {};
  const vendorInput = params.input || {};
  const media = [];
  for (const entry of Array.isArray(vendorInput.media) ? vendorInput.media : []) {
    media.push({ type: entry && entry.type, url: entry && entry.url });
  }
  const parameters = params.parameters || {};
  return validateVideoRequest({
    model: req.model,
    prompt: vendorInput.prompt,
    negative_prompt: vendorInput.negative_prompt,
    media: media,
    duration: parameters.duration,
    resolution: parameters.resolution,
    ratio: parameters.ratio,
    watermark: parameters.watermark,
    seed: parameters.seed,
    prompt_extend: parameters.prompt_extend,
    audio_setting: parameters.audio_setting,
  });
}

// 3D generation runs only on the native route, so the client sends the Vtrix
// params object directly; validation mirrors the vendor field rules.
function threedParamsFromVtrix(params) {
  if (!params || typeof params !== "object" || Array.isArray(params)) throw new Error("params must be an object");
  const generateType = params.generate_type === undefined ? "Normal" : params.generate_type;
  if (THREED_GENERATE_TYPES.indexOf(generateType) < 0)
    throw new Error("generate_type must be one of " + THREED_GENERATE_TYPES.join(", "));
  const prompt = trimmed(params.prompt);
  const imageUrl = trimmed(params.image_url);
  const imageBase64 = trimmed(params.image_base64);
  const inputs = [prompt, imageUrl, imageBase64].filter(function (value) {
    return value;
  }).length;
  if (generateType === "Sketch") {
    if (!prompt || (!imageUrl && !imageBase64)) throw new Error("Sketch mode requires a prompt and a reference image");
    if (params.multi_view_images !== undefined) throw new Error("Sketch mode does not support multi_view_images");
  } else if (inputs !== 1) {
    throw new Error(generateType + " mode requires exactly one of prompt, image_url, or image_base64");
  }
  if (prompt.length > MAX_THREED_PROMPT_RUNES)
    throw new Error("prompt must be at most " + MAX_THREED_PROMPT_RUNES + " characters");
  const flat = { generate_type: generateType };
  if (prompt) flat.prompt = prompt;
  if (imageUrl) {
    if (!/^https?:\/\//i.test(imageUrl)) throw new Error("image_url must be an HTTP URL");
    flat.image_url = imageUrl;
  }
  if (imageBase64) flat.image_base64 = imageBase64;
  const faceCount = optionalInt(params.face_count, "face_count", MIN_FACE_COUNT, MAX_FACE_COUNT);
  if (faceCount !== undefined) flat.face_count = faceCount;
  if (params.polygon_type !== undefined) {
    if (THREED_POLYGON_TYPES.indexOf(params.polygon_type) < 0)
      throw new Error("polygon_type must be triangle or quadrilateral");
    flat.polygon_type = params.polygon_type;
  }
  const enablePbr = optionalBool(params.enable_pbr, "enable_pbr");
  if (enablePbr !== undefined) flat.enable_pbr = enablePbr;
  if (params.multi_view_images !== undefined) {
    if (!Array.isArray(params.multi_view_images)) throw new Error("multi_view_images must be an array");
    const views = [];
    for (const view of params.multi_view_images) {
      if (!view || typeof view !== "object") throw new Error("multi_view_images entries must be objects");
      if (THREED_VIEW_TYPES.indexOf(view.ViewType) < 0)
        throw new Error("multi_view_images ViewType must be one of " + THREED_VIEW_TYPES.join(", "));
      const viewUrl = trimmed(view.ViewImageUrl);
      const viewBase64 = trimmed(view.ViewImageBase64);
      if (!viewUrl && !viewBase64) throw new Error("multi_view_images entries need ViewImageUrl or ViewImageBase64");
      const copy = { ViewType: view.ViewType };
      if (viewUrl) {
        if (!/^https?:\/\//i.test(viewUrl)) throw new Error("ViewImageUrl must be an HTTP URL");
        copy.ViewImageUrl = viewUrl;
      }
      if (viewBase64) copy.ViewImageBase64 = viewBase64;
      views.push(copy);
    }
    flat.multi_view_images = views;
  }
  return flat;
}

function decodeGenerationBody(req) {
  const model = trimmed(req.model);
  if (!model) throw new Error("model is required");
  const input = req.input;
  if (!Array.isArray(input) || input.length !== 1) throw new Error("input must be an array with exactly one entry");
  if (model === "wan26_image") return imageRequestFromVtrix(req);
  if (model === "wan27_i2v") return videoRequestFromVtrix(req);
  if (model === "tencent_hunyuan_3d_pro") return { model: model, params: threedParamsFromVtrix(input[0].params || {}) };
  throw new Error("unsupported model: " + model);
}

function vtrixImageBody(req) {
  const content = [];
  for (const image of req.images || []) content.push({ image: image });
  content.push({ text: req.prompt });
  const parameters = {};
  if (req.interleave) {
    parameters.enable_interleave = true;
    parameters.n = 1;
    if (req.max_images !== undefined && req.max_images !== 5) parameters.max_images = req.max_images;
  } else {
    parameters.n = req.n === undefined ? 1 : req.n;
  }
  if (req.size) parameters.size = req.size;
  if (req.negative_prompt) parameters.negative_prompt = req.negative_prompt;
  if (req.watermark !== undefined) parameters.watermark = req.watermark;
  if (req.seed !== undefined) parameters.seed = req.seed;
  if (!req.interleave && req.prompt_extend !== undefined) parameters.prompt_extend = req.prompt_extend;
  return {
    model: req.model,
    input: [{ params: { input: { messages: [{ role: "user", content: content }] }, parameters: parameters } }],
  };
}

function vtrixVideoBody(req) {
  const input = {};
  if (req.prompt) input.prompt = req.prompt;
  if (req.negative_prompt) input.negative_prompt = req.negative_prompt;
  if (req.media && req.media.length) {
    input.media = req.media.map(function (entry) {
      return { type: entry.type, url: entry.url };
    });
  }
  const parameters = {};
  if (req.duration !== undefined) parameters.duration = req.duration;
  if (req.resolution) parameters.resolution = req.resolution;
  if (req.ratio) parameters.ratio = req.ratio;
  if (req.watermark !== undefined) parameters.watermark = req.watermark;
  if (req.seed !== undefined) parameters.seed = req.seed;
  if (req.prompt_extend !== undefined) parameters.prompt_extend = req.prompt_extend;
  if (req.audio_setting) parameters.audio_setting = req.audio_setting;
  return { model: req.model, input: [{ params: { input: input, parameters: parameters } }] };
}

function vtrixThreedBody(req) {
  return { model: req.model, input: [{ params: req.params }] };
}

function modelOf(ctx) {
  return ctx.upstreamModel || (ctx.requestBody && ctx.requestBody.model) || ctx.model;
}

function authHeader(ctx) {
  return ctx.authHeader || "Bearer " + ctx.apiKey;
}

export function buildSubmitRequest(ctx) {
  const req = ctx.requestBody || {};
  const model = modelOf(ctx);
  let body;
  if (model === "wan26_image") body = vtrixImageBody(Object.assign({}, req, { model: model }));
  else if (model === "wan27_i2v") body = vtrixVideoBody(Object.assign({}, req, { model: model }));
  else if (model === "tencent_hunyuan_3d_pro") body = vtrixThreedBody(Object.assign({}, req, { model: model }));
  else throw new Error("unsupported model: " + String(model));
  return {
    url: ctx.baseUrl + "/model/v1/generation",
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      Authorization: authHeader(ctx),
    },
    body: body,
  };
}

export function parseSubmitResponse(ctx, resp) {
  const body = resp.body || {};
  const error = body.error || {};
  if (error.code || error.message) throw new Error((error.code || "submit_failed") + ": " + (error.message || ""));
  const taskId = trimmed(body.id);
  if (!taskId) throw new Error("missing generation id");
  const req = ctx.requestBody || {};
  let state;
  if (req.params) state = { kind: "threed" };
  else if (req.media) state = { kind: "video" };
  else state = { kind: "image", imageCount: imageCountFor(req) };
  return { taskId: taskId, taskData: body, state: state };
}

export function buildQueryRequest(ctx) {
  return {
    url: ctx.baseUrl + "/model/v1/generation/task/" + ctx.taskId,
    method: "GET",
    headers: { Accept: "application/json", Authorization: authHeader(ctx) },
  };
}

function outputContent(body) {
  const output = body && Array.isArray(body.output) ? body.output : [];
  const entries = [];
  for (const item of output) {
    const content = item && Array.isArray(item.content) ? item.content : [];
    for (const part of content) {
      if (part && typeof part === "object" && trimmed(part.url)) entries.push(part);
    }
  }
  return entries;
}

function contentUrl(entries, type) {
  for (const entry of entries) {
    if ((!type || entry.type === type) && trimmed(entry.url)) return trimmed(entry.url);
  }
  return "";
}

export function parseTaskResult(ctx, body) {
  const status = trimmed(body && body.status);
  const model = modelOf(ctx);
  if (status === "in_progress") return { status: "IN_PROGRESS" };
  if (status === "completed") {
    const entries = outputContent(body);
    if (model === "wan26_image") {
      const images = entries.filter(function (entry) {
        return entry.type === "image";
      });
      if (!images.length) return { status: "FAILURE", reason: "image task completed without any images" };
      return { status: "SUCCESS", url: trimmed(images[0].url) };
    }
    if (model === "wan27_i2v") {
      const url = contentUrl(entries, "video");
      if (!url) return { status: "FAILURE", reason: "video task completed without a video" };
      return { status: "SUCCESS", url: url };
    }
    const url = contentUrl(entries, "glb");
    if (!url) return { status: "FAILURE", reason: "3D task completed without a model file" };
    return { status: "SUCCESS", url: url };
  }
  if (status === "failed") {
    const error = (body && body.error) || {};
    const reason = trimmed(error.message) || trimmed(error.code) || trimmed(body.message) || "task failed";
    return { status: "FAILURE", reason: reason };
  }
  return { status: "UNKNOWN", reason: "unrecognized status: " + status };
}

export function extractUsage(ctx) {
  const req = ctx.requestBody || {};
  if (req.params) {
    return { asset_count: 1 };
  }
  if (req.media) {
    if (ctx.usagePurpose === "billing_ratios") return { seconds: req.duration };
    return { duration: req.duration, resolution: req.resolution };
  }
  const count = imageCountFor(req);
  if (ctx.usagePurpose === "billing_ratios") return { image_count: count };
  return { image_count: count };
}

export function extractUsageOnComplete(ctx, taskResult, body) {
  const req = (ctx && ctx.state) || {};
  if (req.kind !== "image") return {};
  // Vtrix usage.quantity is the actual billed image count; fall back to
  // counting image payloads. Invalid counts keep the reservation in place.
  const entries = outputContent(body);
  const payloadCount = entries.filter(function (entry) {
    return entry.type === "image";
  }).length;
  const usage = (body && body.usage) || {};
  const cap = Number.isInteger(req.imageCount) ? Math.max(req.imageCount, 1) : MAX_IMAGE_OUTPUTS;
  if (usage.quantity !== undefined) {
    const reported = Number(usage.quantity);
    if (!Number.isInteger(reported) || reported < 0 || reported > cap || (payloadCount && reported === 0))
      throw new Error("invalid upstream image count");
    return { image_count: reported };
  }
  if (payloadCount > MAX_IMAGE_OUTPUTS) throw new Error("too many output images");
  if (payloadCount) return { image_count: payloadCount };
  return {};
}

export function listArtifacts(task) {
  if (!task || task.status !== "SUCCESS") return [];
  const entries = outputContent(task.data);
  const images = entries.filter(function (entry) {
    return entry.type === "image";
  });
  if (images.length)
    return images.map(function (_, index) {
      return { key: "image-" + (index + 1), type: "image" };
    });
  return contentUrl(entries, "video") ? [{ key: "video", type: "video" }] : [];
}

export function buildContentRequest(ctx) {
  let url;
  const entries = outputContent((ctx && ctx.data) || {});
  if (ctx.artifactKey === "video") url = contentUrl(entries, "video");
  else {
    const images = entries.filter(function (entry) {
      return entry.type === "image";
    });
    const index = images.findIndex(function (_, position) {
      return ctx.artifactKey === "image-" + (position + 1);
    });
    url = index >= 0 ? trimmed(images[index].url) : "";
  }
  if (!url) throw new Error("artifact_not_found");
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}

export const native = {
  submitGeneration: function (ctx) {
    const req = requireJsonObject(ctx);
    const flat = decodeGenerationBody(req);
    const action = flat.params
      ? "generate_3d_asset"
      : flat.media
        ? "image_to_video"
        : flat.interleave
          ? "image_interleave"
          : "image_edit";
    return { kind: "submit", model: flat.model, action: action, requestBody: flat };
  },
  generationCreated: function (ctx, task) {
    const data = task.data || {};
    return {
      id: task.task_id,
      created_at: data.created_at !== undefined ? data.created_at : Date.now(),
      status: "in_progress",
      error: null,
      model: ctx.upstreamModel || ctx.model,
    };
  },
  generationStatus: function (ctx, task) {
    return task.data || { id: task.task_id, status: "in_progress", error: null };
  },
  error: function (ctx, error) {
    return { code: error.code, message: error.message };
  },
};

export const protocols = {
  openai_image: {
    decodeRequest: function (ctx) {
      return imageRequestFromOpenAI(ctx);
    },
    render: function (ctx, task) {
      const data = [];
      for (const entry of outputContent(task.data)) {
        if (entry.type !== "image") continue;
        const url = trimmed(entry.url);
        if (/^https?:\/\//i.test(url)) data.push({ url: url });
        else if (/^data:[^;,]*;base64,(.*)$/i.test(url)) data.push({ b64_json: url.replace(/^data:[^;,]*;base64,/i, "") });
        else data.push({ b64_json: url });
      }
      return { created: task.created_at, data: data };
    },
  },
  openai_video: {
    decodeRequest: function (ctx) {
      return videoRequestFromOpenAI(ctx);
    },
    render: function (ctx, task) {
      const data = task.data || {};
      const statuses = { in_progress: "in_progress", completed: "completed", failed: "failed" };
      const rawProgress = Number(data.progress);
      const progress = Number.isFinite(rawProgress) && rawProgress >= 0 && rawProgress <= 1 ? Math.round(rawProgress * 100) : 0;
      const output = {
        id: task.task_id,
        object: "video",
        model: "",
        status: statuses[String(data.status || "").toLowerCase()] || "unknown",
        progress: progress,
        created_at: task.created_at,
      };
      if (task.updated_at) output.completed_at = task.updated_at;
      if (String(data.status || "").toLowerCase() === "failed" && data.error)
        output.error = { code: data.error.code || "", message: data.error.message || "" };
      return output;
    },
  },
};