// Output count also respects the host's 64-artifact task limit.
const MAX_IMAGE_OUTPUTS = 64;
const MAX_IMAGE_BYTES = 10485760; // 10 MB input image limit
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
    description: {
      en: "3D asset generation unit price",
      zh: "3D 资产生成单价",
    },
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
const THREED_USAGE_EXAMPLES = [
  { label: "standard", facts: { asset_count: 1 } },
];

const VIDEO_RESOLUTIONS = ["480P", "720P", "1080P"];
const THREED_GENERATE_TYPES = ["Normal", "LowPoly", "Geometry", "Sketch"];
const THREED_POLYGON_TYPES = ["triangle", "quadrilateral"];
const THREED_VIEW_TYPES = [
  "left",
  "right",
  "back",
  "top",
  "bottom",
  "left_front",
  "right_front",
];
const VIDEO_MEDIA_TYPES = [
  "first_frame",
  "last_frame",
  "driving_audio",
  "first_clip",
];

const MODEL_GROUPS = {
  image: [
    "flux_2_max",
    "flux_2_pro",
    "flux_3_image",
    "flux_kontext_max",
    "flux_kontext_pro",
    "flux_pro_11",
    "nano_banana",
    "nano_banana_2",
    "nano_banana_2_1",
    "nano_banana_2_lite",
    "nano_banana_pro",
    "midjourney_V8.1_i2i",
    "midjourney_V8.1_t2i",
    "midjourney_V8.2_i2i",
    "midjourney_V8.2_t2i",
    "midjourney_diffusion",
    "midjourney_edit",
    "midjourney_enhance",
    "midjourney_inpaint",
    "midjourney_outpaint",
    "midjourney_pan",
    "midjourney_remix",
    "midjourney_remove_background",
    "midjourney_retexture",
    "midjourney_uploadpaint",
    "midjourney_upscale",
    "midjourney_variation",
    "face_swap",
    "head_swap",
    "qwen_image_edit_spicy",
    "z_image_spicy",
    "gpt_image_1",
    "gpt_image_1_5",
    "gpt_image_1_5_edit",
    "gpt_image_1_edit",
    "gpt_image_2",
    "gpt_image_2_5_flare",
    "gpt_image_2_5_flare_edit",
    "gpt_image_2_5_sunburst",
    "gpt_image_2_5_sunburst_edit",
    "gpt_image_2_edit",
    "qwen_image_edit_plus",
    "wan25_i2i_preview",
    "wan25_i2i_preview_intl",
    "wan25_t2i_preview",
    "wan25_t2i_preview_intl",
    "wan26_image",
    "wan26_image_intl",
    "wan27_image",
    "wan27_image_intl",
    "wan27_image_pro",
    "wan27_image_pro_intl",
    "tencent_mps_image_super_resolution",
    "viduq2_reference2image",
    "kirin_image_omni",
    "kirin_v1_5_image",
    "kirin_v1_image",
    "kirin_v2_1_image",
    "kirin_v2_1_reference2image",
    "kirin_v2_image",
    "kirin_v2_reference2image",
    "kirin_v3_image",
    "kirin_v3_omni_image",
    "spark_dream_3_0",
    "spark_dream_4_0",
    "spark_dream_4_5",
    "spark_dream_5_0",
    "spark_dream_5_0_flash",
    "spark_dream_5_0_lite",
    "spark_dream_5_0_pro",
    "spark_dreamo",
    "spark_edit_portrait",
    "spark_i2i_3_0",
    "spark_image_3_1",
    "spark_img_effects",
    "spark_multi_dreamo",
    "spark_style_template",
    "spark_t2i_3_0",
    "grok_imagine_image",
    "grok_imagine_image_quality",
    "grok_imagine_image_quality_edit",
  ],
  video: [
    "gemini_omni_flash_preview",
    "veo_3.1_generate_001",
    "veo_31_fast",
    "vtrix-veo-3.1-generate-preview",
    "minimax_h3",
    "minimax_h3_regeneration",
    "minimax_hailuo_02_i2v",
    "minimax_hailuo_02_t2v",
    "minimax_hailuo_23_fast_i2v",
    "minimax_hailuo_23_i2v",
    "minimax_hailuo_23_t2v",
    "minimax_i2v_01",
    "minimax_i2v_01_director",
    "minimax_i2v_01_live",
    "minimax_t2v_01",
    "minimax_t2v_01_director",
    "wan22_i2v_spicy",
    "wan27_i2v_spicy",
    "sora_2_i2v",
    "sora_2_pro_i2v",
    "sora_2_pro_t2v",
    "sora_2_t2v",
    "vtrix-sora-2",
    "vtrix-sora-2-pro",
    "happyhorse_1.0_i2v",
    "happyhorse_1.0_r2v",
    "happyhorse_1.0_t2v",
    "happyhorse_1.0_video_edit",
    "happyhorse_1.1_i2v",
    "happyhorse_1.1_r2v",
    "happyhorse_1.1_t2v",
    "wan22_i2v_flash",
    "wan22_i2v_plus",
    "wan22_t2v_plus",
    "wan25_i2v_preview",
    "wan25_i2v_preview_intl",
    "wan25_t2v_preview",
    "wan25_t2v_preview_intl",
    "wan26_i2v",
    "wan26_r2v",
    "wan26_t2v",
    "wan27_i2v",
    "wan27_i2v_intl",
    "wan27_r2v",
    "wan27_r2v_intl",
    "wan27_t2v",
    "wan27_t2v_intl",
    "wan27_videoedit",
    "wan27_videoedit_intl",
    "wan30_video",
    "wan30_video_intl",
    "wan30_video_prime",
    "wan30_video_prime_intl",
    "tencent_mps_super_resolution",
    "vidu-cn-start-end2video-viduq1",
    "vidu-cn-start-end2video-viduq1-classic",
    "vidu20_img2video",
    "viduq1_img2video",
    "viduq1_reference2video",
    "viduq1_text2video",
    "viduq2_pro_extend",
    "viduq2_pro_fast_img2video",
    "viduq2_pro_fast_reference2video",
    "viduq2_pro_fast_start_end2video",
    "viduq2_pro_fast_text2video",
    "viduq2_pro_img2video",
    "viduq2_pro_reference",
    "viduq2_pro_start_end2video",
    "viduq2_reference2video",
    "viduq2_text2video",
    "viduq2_turbo_extend",
    "viduq2_turbo_img2video",
    "viduq2_turbo_start_end2video",
    "viduq3_i2v_reference",
    "viduq3_pro_img2video",
    "viduq3_pro_start_end2video",
    "viduq3_pro_text2video",
    "viduq3_turbo_img2video",
    "viduq3_turbo_start_end2video",
    "viduq3_turbo_text2video",
    "viduq4_preview_img2video",
    "viduq4_preview_reference2video",
    "doubao_seedance_2_0",
    "kirin_duration_extension",
    "kirin_lipsync",
    "kirin_motion_control",
    "kirin_v1_5_i2v",
    "kirin_v1_6_i2v",
    "kirin_v1_6_multi_i2v",
    "kirin_v1_6_t2v",
    "kirin_v1_i2v",
    "kirin_v1_t2v",
    "kirin_v2_1_i2v",
    "kirin_v2_1_master_i2v",
    "kirin_v2_1_master_t2v",
    "kirin_v2_5_turbo_i2v",
    "kirin_v2_5_turbo_t2v",
    "kirin_v2_6_i2v",
    "kirin_v2_6_motion_control",
    "kirin_v2_6_t2v",
    "kirin_v2_master_i2v",
    "kirin_v2_master_t2v",
    "kirin_v3_i2v",
    "kirin_v3_motion_control",
    "kirin_v3_omni_video",
    "kirin_v3_t2v",
    "kirin_v3_turbo_i2v",
    "kirin_v3_turbo_t2v",
    "kirin_v4_i2v",
    "kirin_v4_omni_video",
    "kirin_v4_t2v",
    "kirin_video_o1",
    "spark_actor_imitation",
    "spark_avatar_imitator_v2v",
    "spark_avatar_picture_omni_v2",
    "spark_dance_v1_0_lite_i2v",
    "spark_dance_v1_0_lite_t2v",
    "spark_dance_v1_0_pro",
    "spark_dance_v1_0_pro_fast",
    "spark_dance_v1_5_pro",
    "spark_dance_v2_0",
    "spark_dance_v2_0_fast",
    "spark_dance_v2_0_mini",
  ],
  model: [
    "hi3d_image_to_3d",
    "hi3d_image_to_relief",
    "hi3d_model_multicolor",
    "hi3d_model_split",
    "tencent_hunyuan_3d",
    "tencent_hunyuan_3d_pro",
    "tencent_hunyuan_3d_pro_intl",
    "tencent_hunyuan_3d_rapid",
    "tencent_hunyuan_3d_rapid_intl",
    "spark_3d",
  ],
  voice: [
    "gemini_3_1_flash_tts_preview",
    "minimax_speech_01_hd",
    "minimax_speech_01_turbo",
    "minimax_speech_02_hd",
    "minimax_speech_02_turbo",
    "minimax_speech_26_hd",
    "minimax_speech_26_turbo",
    "minimax_speech_28_hd",
    "minimax_speech_28_turbo",
    "kirin_text_to_audio",
    "kirin_tts",
    "kirin_video_to_audio",
  ],
};
const MODEL_KINDS = {};
for (const kind of Object.keys(MODEL_GROUPS)) {
  for (const model of MODEL_GROUPS[kind]) MODEL_KINDS[model] = kind;
}
const REQUEST_USAGE_SCHEMA = {
  requests: {
    type: "number",
    unit: "count",
    unitLabel: { en: "task", zh: "次", "zh-TW": "次" },
    description: { en: "Generation task unit price", zh: "生成任务单价" },
  },
};
const SPEECH_USAGE_SCHEMA = {
  characters: {
    type: "number",
    unit: "credit",
    description: { en: "Speech character unit price", zh: "语音字符单价" },
  },
};
const AUDIO_USAGE_SCHEMA = {
  duration: {
    type: "number",
    unit: "second",
    description: { en: "Audio generation unit price", zh: "音频生成单价" },
  },
};
const FLUX_IMAGE_USAGE_SCHEMA = Object.assign({}, IMAGE_USAGE_SCHEMA, {
  resolution: {
    enum: ["768sq", "1k", "1.5k", "2k", "4k"],
    description: { en: "Output image resolution", zh: "输出图片分辨率" },
  },
});
const SPEECH_MODELS = MODEL_GROUPS.voice.filter(function (model) {
  return model.indexOf("minimax_speech_") === 0 || model === "kirin_tts";
});
const TASK_PRICED_MODELS = MODEL_GROUPS.video
  .filter(function (model) {
    return model !== "wan27_i2v";
  })
  .concat(
    MODEL_GROUPS.image.filter(function (model) {
      return (
        model.indexOf("midjourney_") === 0 ||
        model.indexOf("nano_banana") === 0 ||
        (model.indexOf("flux_") === 0 && model !== "flux_3_image") ||
        [
          "face_swap",
          "head_swap",
          "qwen_image_edit_spicy",
          "z_image_spicy",
          "viduq2_reference2image",
          "spark_dream_3_0",
          "spark_dreamo",
          "spark_edit_portrait",
          "spark_i2i_3_0",
          "spark_image_3_1",
          "spark_img_effects",
          "spark_multi_dreamo",
          "spark_style_template",
          "spark_t2i_3_0",
        ].indexOf(model) >= 0
      );
    }),
    ["gemini_3_1_flash_tts_preview", "kirin_video_to_audio"],
  );
const IMAGE_ADAPTERS = {
  flux_2_max: { kind: "flux", maxImages: 8 },
  flux_2_pro: { kind: "flux", maxImages: 8 },
  flux_3_image: { kind: "flux3", maxImages: 10 },
  flux_kontext_max: { kind: "flux", maxImages: 4 },
  flux_kontext_pro: { kind: "flux", maxImages: 4 },
  flux_pro_11: { kind: "flux", maxImages: 0 },
  gpt_image_1: { kind: "gpt", edit: false },
  gpt_image_1_5: { kind: "gpt", edit: false },
  gpt_image_1_5_edit: { kind: "gpt", edit: true },
  gpt_image_1_edit: { kind: "gpt", edit: true },
  gpt_image_2: { kind: "gpt", edit: false },
  gpt_image_2_5_flare: { kind: "gpt", edit: false },
  gpt_image_2_5_flare_edit: { kind: "gpt", edit: true },
  gpt_image_2_5_sunburst: { kind: "gpt", edit: false },
  gpt_image_2_5_sunburst_edit: { kind: "gpt", edit: true },
  gpt_image_2_edit: { kind: "gpt", edit: true },
  grok_imagine_image: { kind: "grok", edit: false },
  grok_imagine_image_quality: { kind: "grok", edit: false },
  grok_imagine_image_quality_edit: { kind: "grok", edit: true },
  kirin_image_omni: { kind: "kirin", maxImages: 10, omni: true },
  kirin_v1_5_image: { kind: "kirin", maxImages: 1, omni: false },
  kirin_v1_image: { kind: "kirin", maxImages: 1, omni: false },
  kirin_v2_1_image: { kind: "kirin", maxImages: 1, omni: false },
  kirin_v2_image: { kind: "kirin", maxImages: 1, omni: false },
  kirin_v3_image: { kind: "kirin", maxImages: 1, omni: false },
  kirin_v3_omni_image: { kind: "kirin", maxImages: 10, omni: true },
  nano_banana: { kind: "nano", jsonOnly: true },
  nano_banana_2: { kind: "nano", jsonOnly: true },
  nano_banana_2_1: { kind: "nano", jsonOnly: true },
  nano_banana_2_lite: { kind: "nano", jsonOnly: true },
  nano_banana_pro: { kind: "nano", jsonOnly: true },
  qwen_image_edit_plus: { kind: "messages", countMax: 6, edit: true },
  spark_dream_4_0: { kind: "spark", maxImages: 14, series: true },
  spark_dream_4_5: { kind: "spark", maxImages: 14, series: true },
  spark_dream_5_0: { kind: "spark", maxImages: 14, series: true },
  spark_dream_5_0_flash: { kind: "spark", maxImages: 10, layers: true },
  spark_dream_5_0_lite: { kind: "spark", maxImages: 14, series: true },
  spark_dream_5_0_pro: { kind: "spark", maxImages: 10, layers: true },
  wan25_i2i_preview: { kind: "wan25", countMax: 4, edit: true },
  wan25_i2i_preview_intl: { kind: "wan25", countMax: 4, edit: true },
  wan25_t2i_preview: { kind: "wan25", countMax: 4, edit: false },
  wan25_t2i_preview_intl: { kind: "wan25", countMax: 4, edit: false },
  wan26_image: { kind: "wan26", countMax: 4 },
  wan26_image_intl: { kind: "wan26", countMax: 4 },
  wan27_image: { kind: "messages", countMax: MAX_IMAGE_OUTPUTS },
  wan27_image_intl: { kind: "messages", countMax: MAX_IMAGE_OUTPUTS },
  wan27_image_pro: { kind: "messages", countMax: MAX_IMAGE_OUTPUTS },
  wan27_image_pro_intl: { kind: "messages", countMax: MAX_IMAGE_OUTPUTS },
};
const OPENAI_IMAGE_MODELS = Object.keys(IMAGE_ADAPTERS);
const VIDEO_PARAMETER_GROUPS = [
  {
    models: [
      "doubao_seedance_2_0",
      "spark_dance_v2_0",
      "spark_dance_v2_0_fast",
      "spark_dance_v2_0_mini",
    ],
    duration: { auto: [-1], default: 5, max: 15, min: 4, path: "duration" },
  },
  {
    models: [
      "happyhorse_1.0_i2v",
      "happyhorse_1.0_r2v",
      "happyhorse_1.0_t2v",
      "happyhorse_1.1_i2v",
      "happyhorse_1.1_r2v",
      "happyhorse_1.1_t2v",
      "kirin_v3_turbo_i2v",
      "kirin_v3_turbo_t2v",
    ],
    duration: { default: 5, max: 15, min: 3, path: "duration" },
  },
  {
    models: [
      "kirin_v1_5_i2v",
      "kirin_v1_6_i2v",
      "kirin_v1_6_multi_i2v",
      "kirin_v1_6_t2v",
      "kirin_v1_i2v",
      "kirin_v1_t2v",
      "kirin_v2_1_i2v",
      "kirin_v2_1_master_i2v",
      "kirin_v2_5_turbo_i2v",
      "kirin_v2_5_turbo_t2v",
      "kirin_v2_6_i2v",
      "kirin_v2_6_t2v",
      "kirin_v2_master_i2v",
      "kirin_v2_1_master_t2v",
      "kirin_v2_master_t2v",
    ],
    duration: {
      default: 5,
      max: 10,
      min: 5,
      path: "duration",
      values: [5, 10],
    },
  },
  {
    models: ["kirin_v3_i2v", "kirin_v3_t2v", "kirin_v3_omni_video"],
    duration: { default: 5, max: 15, min: 3, path: "duration" },
    shots: 6,
  },
  {
    models: ["kirin_v4_i2v", "kirin_v4_omni_video", "kirin_v4_t2v"],
    duration: { default: 10, max: 30, min: 3, path: "settings.duration" },
  },
  {
    models: ["kirin_video_o1"],
    duration: { default: 5, max: 10, min: 3, path: "duration" },
  },
  { models: ["minimax_h3"], duration: { max: 15, min: 4, path: "duration" } },
  { models: ["minimax_h3_regeneration"], forbidden: ["duration", "ratio"] },
  {
    models: [
      "minimax_hailuo_02_i2v",
      "minimax_hailuo_02_t2v",
      "minimax_hailuo_23_fast_i2v",
      "minimax_hailuo_23_i2v",
      "minimax_hailuo_23_t2v",
    ],
    duration: {
      default: 6,
      max: 10,
      min: 6,
      path: "duration",
      values: [6, 10],
    },
  },
  {
    models: [
      "minimax_i2v_01",
      "minimax_i2v_01_director",
      "minimax_i2v_01_live",
      "minimax_t2v_01",
      "minimax_t2v_01_director",
    ],
    duration: { default: 6, max: 6, min: 6, path: "duration", values: [6] },
  },
  {
    models: ["sora_2_i2v", "sora_2_pro_i2v", "sora_2_pro_t2v", "sora_2_t2v"],
    duration: {
      default: 4,
      max: 12,
      min: 4,
      path: "seconds",
      values: [4, 8, 12],
    },
  },
  {
    models: [
      "spark_dance_v1_0_lite_i2v",
      "spark_dance_v1_0_lite_t2v",
      "spark_dance_v1_0_pro",
      "spark_dance_v1_0_pro_fast",
    ],
    duration: { default: 5, max: 12, min: 2, path: "duration" },
    frames: { min: 29, max: 289, base: 25, step: 4 },
  },
  {
    models: ["spark_dance_v1_5_pro"],
    duration: { auto: [-1], default: 5, max: 12, min: 4, path: "duration" },
  },
  {
    models: ["veo_3.1_generate_001"],
    batch: { path: "sampleCount", max: 4 },
    duration: {
      default: 8,
      max: 8,
      min: 4,
      path: "durationSeconds",
      values: [4, 6, 8],
    },
  },
  {
    models: ["veo_31_fast"],
    duration: { max: 8, min: 4, path: "durationSeconds", values: [4, 6, 8] },
  },
  {
    models: ["vidu20_img2video"],
    duration: { default: 4, max: 8, min: 4, path: "duration", values: [4, 8] },
  },
  {
    models: ["viduq1_img2video", "viduq1_reference2video", "viduq1_text2video"],
    duration: { default: 5, max: 5, min: 5, path: "duration", values: [5] },
  },
  {
    models: ["viduq2_pro_extend", "viduq2_turbo_extend"],
    duration: { default: 5, max: 7, min: 1, path: "duration" },
  },
  {
    models: [
      "viduq2_pro_fast_img2video",
      "viduq2_pro_img2video",
      "viduq2_reference2video",
      "viduq2_text2video",
      "viduq2_turbo_img2video",
    ],
    duration: { default: 5, max: 10, min: 1, path: "duration" },
  },
  {
    models: [
      "viduq2_pro_fast_reference2video",
      "viduq2_pro_fast_start_end2video",
      "viduq2_pro_fast_text2video",
      "viduq2_pro_start_end2video",
      "viduq2_turbo_start_end2video",
    ],
    duration: { default: 5, max: 8, min: 1, path: "duration" },
  },
  {
    models: ["viduq2_pro_reference"],
    duration: { auto: [0], default: 5, max: 10, min: 1, path: "duration" },
  },
  {
    models: [
      "viduq3_i2v_reference",
      "viduq4_preview_img2video",
      "viduq4_preview_reference2video",
    ],
    duration: { default: 5, max: 16, min: 3, path: "duration" },
  },
  {
    models: [
      "viduq3_pro_img2video",
      "viduq3_pro_start_end2video",
      "viduq3_pro_text2video",
      "viduq3_turbo_img2video",
      "viduq3_turbo_start_end2video",
      "viduq3_turbo_text2video",
    ],
    duration: { default: 5, max: 16, min: 1, path: "duration" },
  },
  {
    models: ["vtrix-sora-2", "vtrix-sora-2-pro"],
    duration: { max: 8, min: 4, path: "seconds", values: [4, 8] },
  },
  {
    models: ["vtrix-veo-3.1-generate-preview"],
    duration: { max: 8, min: 8, path: "seconds", values: [8] },
  },
  {
    models: ["wan22_i2v_flash", "wan22_i2v_plus", "wan22_t2v_plus"],
    duration: {
      default: 5,
      max: 5,
      min: 5,
      path: "parameters.duration",
      values: [5],
    },
  },
  {
    models: ["wan22_i2v_spicy"],
    duration: { default: 5, max: 8, min: 5, path: "duration", values: [5, 8] },
  },
  {
    models: [
      "wan25_i2v_preview",
      "wan25_i2v_preview_intl",
      "wan25_t2v_preview",
      "wan25_t2v_preview_intl",
    ],
    duration: {
      default: 5,
      max: 10,
      min: 5,
      path: "parameters.duration",
      values: [5, 10],
    },
  },
  {
    models: ["wan26_i2v", "wan26_r2v", "wan26_t2v"],
    duration: { default: 5, max: 15, min: 2, path: "parameters.duration" },
  },
  {
    models: [
      "wan27_i2v",
      "wan27_i2v_intl",
      "wan27_r2v",
      "wan27_r2v_intl",
      "wan27_t2v",
      "wan27_t2v_intl",
      "wan27_videoedit",
      "wan27_videoedit_intl",
    ],
    duration: { default: 5, max: 3600, min: 1, path: "parameters.duration" },
  },
  {
    models: ["wan27_i2v_spicy"],
    duration: { default: 5, max: 15, min: 2, path: "duration" },
  },
  {
    models: [
      "wan30_video",
      "wan30_video_intl",
      "wan30_video_prime",
      "wan30_video_prime_intl",
    ],
    duration: {
      auto: [-1],
      default: 5,
      max: 30,
      min: 2,
      path: "parameters.duration",
    },
  },
];
const VIDEO_PARAMETER_RULES = {};
for (const group of VIDEO_PARAMETER_GROUPS) {
  for (const model of group.models) VIDEO_PARAMETER_RULES[model] = group;
}

export const meta = {
  apiVersion: 1,
  key: "vtrix",
  name: "Vtrix",
  description: {
    en: "Vtrix image generation and editing, video, 3D, and audio generation",
    zh: "Vtrix 图片生成与编辑、视频、3D 及音频生成",
  },
  version: "1.1.0",
  author: { name: "afeng" },
  baseUrl: "https://cloud.vtrix.ai",
  models: Object.keys(MODEL_KINDS),
  fetchMode: "per_task",
  usageSchema: IMAGE_USAGE_SCHEMA,
  usageProfiles: [
    {
      models: ["wan27_i2v"],
      schema: VIDEO_USAGE_SCHEMA,
      examples: VIDEO_USAGE_EXAMPLES,
    },
    {
      models: MODEL_GROUPS.model,
      schema: THREED_USAGE_SCHEMA,
      examples: THREED_USAGE_EXAMPLES,
    },
    { models: TASK_PRICED_MODELS, schema: REQUEST_USAGE_SCHEMA },
    { models: SPEECH_MODELS, schema: SPEECH_USAGE_SCHEMA },
    { models: ["kirin_text_to_audio"], schema: AUDIO_USAGE_SCHEMA },
    { models: ["flux_3_image"], schema: FLUX_IMAGE_USAGE_SCHEMA },
  ],
  usageExamples: IMAGE_USAGE_EXAMPLES,
  routes: [
    {
      method: "POST",
      path: "/vtrix/v1/generation",
      type: "submit",
      models: Object.keys(MODEL_KINDS),
      decode: "submitGeneration",
      render: "generationCreated",
    },
    {
      method: "GET",
      path: "/vtrix/v1/generation/task/:task_id",
      type: "query",
      render: "generationStatus",
    },
  ],
  protocols: [
    { name: "openai_image", models: OPENAI_IMAGE_MODELS },
    { name: "openai_video", models: ["wan27_i2v"] },
  ],
};

function trimmed(value) {
  return String(value === undefined || value === null ? "" : value).trim();
}

function isImageInput(value) {
  return (
    /^https?:\/\//i.test(trimmed(value)) ||
    /^data:[^;,]*;base64,/i.test(trimmed(value))
  );
}

function filePlaceholder(ref) {
  return { __fileRef: ref, encoding: "dataUrl", maxBytes: MAX_IMAGE_BYTES };
}

function requireJsonObject(ctx) {
  if (
    !ctx.body ||
    ctx.body.kind !== "json" ||
    !ctx.body.value ||
    typeof ctx.body.value !== "object" ||
    Array.isArray(ctx.body.value)
  )
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
  if (typeof value !== "number" && typeof value !== "string")
    throw new Error(
      name + " must be an integer between " + min + " and " + max,
    );
  const n = Number(value);
  if (!Number.isInteger(n) || n < min || n > max)
    throw new Error(
      name + " must be an integer between " + min + " and " + max,
    );
  return n;
}

function modelKind(model) {
  if (!Object.prototype.hasOwnProperty.call(MODEL_KINDS, model))
    throw new Error("unsupported model: " + String(model));
  return MODEL_KINDS[model];
}

function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function boundedNumber(value, name, min, max, integer) {
  if (
    (typeof value !== "number" && typeof value !== "string") ||
    trimmed(value) === ""
  )
    throw new Error(name + " must be a number");
  const number = Number(value);
  if (
    !Number.isFinite(number) ||
    number < min ||
    number > max ||
    (integer && !Number.isInteger(number))
  )
    throw new Error(
      name +
        " must be " +
        (integer ? "an integer" : "a number") +
        " between " +
        min +
        " and " +
        max,
    );
  return number;
}

function generationParams(body, model) {
  if (
    ["wan26_i2v", "wan26_r2v", "wan26_t2v"].indexOf(model) >= 0 &&
    isObject(body.input)
  )
    return { input: body.input, parameters: body.parameters || {} };
  if (!Array.isArray(body.input) || body.input.length !== 1)
    throw new Error("input must be an array with exactly one entry");
  if (!isObject(body.input[0]) || !isObject(body.input[0].params))
    throw new Error("input[0].params must be an object");
  return body.input[0].params;
}

function validateBillingParameters(value, model) {
  if (!value || typeof value !== "object") return;
  for (const key of Object.keys(value)) {
    const field = value[key];
    if (
      [
        "n",
        "max_images",
        "num_images",
        "num_outputs",
        "num_videos",
        "batch_size",
        "series_amount",
        "sampleCount",
      ].indexOf(key) >= 0
    )
      boundedNumber(
        field,
        key,
        1,
        key === "sampleCount" ? 4 : MAX_IMAGE_OUTPUTS,
        true,
      );
    if (["duration", "seconds", "durationSeconds"].indexOf(key) >= 0) {
      const auto =
        Number(field) === -1 &&
        (model.indexOf("wan30_") === 0 ||
          model === "spark_dance_v1_5_pro" ||
          model.indexOf("spark_dance_v2_0") === 0 ||
          model === "doubao_seedance_2_0");
      const automaticVidu =
        model === "viduq2_pro_reference" && Number(field) === 0;
      if (!auto && !automaticVidu)
        boundedNumber(field, key, 0.001, MAX_DURATION_SECONDS, false);
    }
    if (
      [
        "max_tokens",
        "max_completion_tokens",
        "max_output_tokens",
        "maxOutputTokens",
      ].indexOf(key) >= 0
    )
      boundedNumber(field, key, 0, 1073741823, true);
    if (field && typeof field === "object")
      validateBillingParameters(field, model);
  }
}

function validateVideoParameters(model, params) {
  const config = VIDEO_PARAMETER_RULES[model] || {};
  for (const field of config.forbidden || []) {
    if (Object.prototype.hasOwnProperty.call(params, field))
      throw new Error(model + " does not support " + field);
  }
  const pending = [{ value: params, path: "" }];
  let requests = 1;
  while (pending.length) {
    const item = pending.pop();
    for (const field of Object.keys(item.value)) {
      const value = item.value[field];
      const path = item.path ? item.path + "." + field : field;
      if (value && typeof value === "object") {
        pending.push({ value: value, path: path });
        continue;
      }
      const lower = field.toLowerCase();
      const isDuration =
        [
          "duration",
          "seconds",
          "durationseconds",
          "video_duration",
          "target_duration",
          "duration_sec",
        ].indexOf(lower) >= 0;
      const isCount =
        [
          "n",
          "samplecount",
          "sample_count",
          "batch_size",
          "batch_count",
          "num_videos",
          "num_outputs",
          "number_of_videos",
          "number_of_samples",
          "quantity",
        ].indexOf(lower) >= 0;
      if (!isDuration && !isCount && lower !== "frames") continue;
      const number = boundedNumber(value, path, -1, 86400, true);
      if (isDuration) {
        const duration =
          config.duration && path === config.duration.path
            ? config.duration
            : null;
        if (duration && (duration.auto || []).indexOf(number) >= 0) continue;
        const min = duration ? duration.min : 1;
        const max = duration ? duration.max : MAX_DURATION_SECONDS;
        if (
          number < min ||
          number > max ||
          (duration && duration.values && duration.values.indexOf(number) < 0)
        )
          throw new Error(path + " is outside the supported duration");
      } else if (lower === "frames") {
        const frames =
          config.frames && path === "frames" ? config.frames : null;
        if (
          number < (frames ? frames.min : 1) ||
          number > (frames ? frames.max : 86400) ||
          (frames && (number - frames.base) % frames.step !== 0)
        )
          throw new Error(path + " is outside the supported frame count");
      } else {
        const batch =
          config.batch && path === config.batch.path ? config.batch : null;
        if (number < 1 || number > (batch ? batch.max : 1))
          throw new Error(path + " is outside the supported batch count");
        if (batch) requests = number;
      }
    }
  }
  if (config.shots && params.multi_shot === true) {
    const shots = params.multi_prompt;
    if (
      !Array.isArray(shots) ||
      shots.length < 1 ||
      shots.length > config.shots
    )
      throw new Error(
        "multi_prompt must contain between 1 and " + config.shots + " shots",
      );
    const total =
      params.duration === undefined
        ? config.duration.default
        : Number(params.duration);
    let sum = 0;
    for (const shot of shots) {
      if (!isObject(shot))
        throw new Error("multi_prompt entries must be objects");
      sum += boundedNumber(shot.duration, "shot duration", 1, total, true);
    }
    if (sum !== total)
      throw new Error("shot durations must sum to the total duration");
  }
  return { requests: requests };
}

function nativeImageCount(model, params) {
  const options = params.parameters || {};
  if (model === "flux_3_image") return 1;
  if (model === "spark_dream_5_0_flash" || model === "spark_dream_5_0_pro")
    return params.layer_decomposition === true ? 17 : 1;
  if (model === "kirin_v3_omni_image" && params.result_type === "series")
    return boundedNumber(
      params.series_amount === undefined ? 4 : params.series_amount,
      "series_amount",
      2,
      9,
      true,
    );
  if (
    model.indexOf("spark_dream_") === 0 &&
    params.sequential_image_generation === "auto"
  ) {
    const sequential = params.sequential_image_generation_options || {};
    const count = boundedNumber(
      sequential.max_images === undefined ? 15 : sequential.max_images,
      "max_images",
      1,
      15,
      true,
    );
    const references =
      params.image === undefined ? [] : [].concat(params.image);
    if (references.length >= 15)
      throw new Error(
        "sequential generation accepts at most 14 reference images",
      );
    return Math.min(count, 15 - references.length);
  }
  if (model === "wan26_image" || model === "wan26_image_intl") {
    if (options.enable_interleave === true) {
      if (options.n !== undefined && Number(options.n) !== 1)
        throw new Error("interleaved generation requires n to be 1");
      return boundedNumber(
        options.max_images === undefined ? 5 : options.max_images,
        "max_images",
        1,
        5,
        true,
      );
    }
    return boundedNumber(
      options.n === undefined ? 1 : options.n,
      "n",
      1,
      4,
      true,
    );
  }
  if (model.indexOf("wan25_") === 0)
    return boundedNumber(
      options.n === undefined ? 4 : options.n,
      "n",
      1,
      4,
      true,
    );
  if (model.indexOf("wan27_image") === 0 || model === "qwen_image_edit_plus")
    return boundedNumber(
      options.n === undefined ? 1 : options.n,
      "n",
      1,
      model === "qwen_image_edit_plus" ? 6 : MAX_IMAGE_OUTPUTS,
      true,
    );
  const max =
    model.indexOf("gpt_image_") === 0
      ? 4
      : model.indexOf("kirin_") === 0
        ? 9
        : MAX_IMAGE_OUTPUTS;
  return boundedNumber(
    params.n === undefined ? 1 : params.n,
    "n",
    1,
    max,
    true,
  );
}

function validateNativeGeneration(body, model) {
  const kind = modelKind(model);
  const copy = utils.json.clone(body);
  copy.model = model;
  const params = generationParams(copy, model);
  if (params.parameters !== undefined && !isObject(params.parameters))
    throw new Error("parameters must be an object");
  if (model.indexOf("wan27_image") === 0) {
    if (!params.parameters) params.parameters = {};
    if (params.parameters.n === undefined) params.parameters.n = 1;
  }
  if (model.indexOf("grok_imagine_image") === 0 && params.n === undefined)
    params.n = 1;
  if (model === "wan27_i2v") {
    if (!params.parameters) params.parameters = {};
    if (params.parameters.duration === undefined)
      params.parameters.duration = 5;
    params.parameters.resolution = normalizeVideoResolution(
      params.parameters.resolution,
    );
  }
  if (model === "flux_3_image") {
    const sizes = {
      "1024x1024": ["1k", "1:1"],
      "1792x1024": ["1.5k", "16:9"],
      "2048x2048": ["2k", "1:1"],
    };
    const size = trimmed(params.size).replace(/[*×]/g, "x");
    const derived = sizes[size];
    if (
      size &&
      !derived &&
      (!trimmed(params.resolution) || !trimmed(params.aspect_ratio))
    )
      throw new Error(
        "size requires a documented mapping or explicit resolution and aspect_ratio",
      );
    const resolution = trimmed(
      params.resolution || (derived ? derived[0] : "1k"),
    ).toLowerCase();
    if (FLUX_IMAGE_USAGE_SCHEMA.resolution.enum.indexOf(resolution) < 0)
      throw new Error("unsupported image resolution");
    params.resolution = resolution;
    if (derived && params.aspect_ratio === undefined)
      params.aspect_ratio = derived[1];
  }
  validateBillingParameters(params, model);
  if (kind === "video") validateVideoParameters(model, params);
  if (kind === "image") nativeImageCount(model, params);
  if (model.indexOf("midjourney_") === 0) {
    for (const field of ["text", "prompt", "remix_prompt", "remixPrompt"]) {
      const prompt = trimmed(params[field]);
      if (
        /--(?:repeat|r)(?:\s|=|$)/i.test(prompt) ||
        /\{[^{}]*,[^{}]*\}/.test(prompt)
      )
        throw new Error(
          "Midjourney repeat and permutation prompts require separate generation requests",
        );
    }
  }
  if (model.indexOf("minimax_speech_") === 0 || model === "kirin_tts") {
    if (typeof params.text !== "string" || !params.text.trim())
      throw new Error("text is required");
    const limit = model === "kirin_tts" ? 1000 : 9999;
    if (Array.from(params.text).length > limit)
      throw new Error("text must be at most " + limit + " characters");
    if (params.stream === true || params.stream === "true")
      throw new Error("streaming audio is not supported by the task interface");
    if (model === "kirin_tts") {
      if (!trimmed(params.voice_id)) throw new Error("voice_id is required");
      if (["zh", "en"].indexOf(params.voice_language) < 0)
        throw new Error("voice_language must be zh or en");
      if (params.voice_speed !== undefined)
        boundedNumber(params.voice_speed, "voice_speed", 0.8, 2, false);
    }
  }
  if (
    model === "gemini_3_1_flash_tts_preview" &&
    !trimmed(params.text || params.prompt)
  )
    throw new Error("text is required");
  if (model === "kirin_text_to_audio") {
    if (!trimmed(params.prompt) || Array.from(params.prompt).length > 200)
      throw new Error("prompt must contain 1 to 200 characters");
    const duration = boundedNumber(params.duration, "duration", 3, 10, false);
    if (Math.abs(duration * 10 - Math.round(duration * 10)) > 0.000001)
      throw new Error("duration supports one decimal place");
  }
  if (model === "kirin_video_to_audio") {
    if (
      Boolean(trimmed(params.video_url)) === Boolean(trimmed(params.video_id))
    )
      throw new Error("exactly one of video_url or video_id is required");
    for (const field of ["sound_effect_prompt", "bgm_prompt"])
      if (
        params[field] !== undefined &&
        Array.from(String(params[field])).length > 200
      )
        throw new Error(field + " must be at most 200 characters");
  }
  if (model === "tencent_hunyuan_3d_pro") threedParamsFromVtrix(params);
  return copy;
}

function nativeUsage(body, model) {
  const params = generationParams(body, model);
  const kind = modelKind(model);
  if (kind === "model") return { asset_count: 1 };
  if (SPEECH_MODELS.indexOf(model) >= 0)
    return { characters: Array.from(params.text).length };
  if (model === "kirin_text_to_audio")
    return { duration: Number(params.duration) };
  if (model === "flux_3_image")
    return { image_count: 1, resolution: params.resolution };
  if (model === "wan27_i2v") {
    const options = params.parameters || {};
    return {
      duration: boundedNumber(
        options.duration === undefined ? 5 : options.duration,
        "duration",
        1,
        MAX_DURATION_SECONDS,
        false,
      ),
      resolution: normalizeVideoResolution(options.resolution),
    };
  }
  if (TASK_PRICED_MODELS.indexOf(model) >= 0)
    return kind === "video"
      ? validateVideoParameters(model, params)
      : { requests: 1 };
  return { image_count: nativeImageCount(model, params) };
}

function generationURL(ctx) {
  return (
    trimmed(ctx.baseUrl)
      .replace(/\/+$/, "")
      .replace(/\/model$/, "") + "/model/v1/generation"
  );
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

function imageRequestFromOpenAI(ctx) {
  const model = ctx.upstreamModel || ctx.model;
  const adapter = IMAGE_ADAPTERS[model];
  if (!adapter)
    throw new Error("this model requires the native Vtrix generation endpoint");
  let req;
  const uploads = [];
  let uploadedMask;
  const formImages = [];
  let hasFormImages = false;
  const multipart = ctx.body && ctx.body.kind === "multipart";
  if (ctx.body && ctx.body.kind === "json") {
    if (!isObject(ctx.body.value))
      throw new Error("request body must be an object");
    req = ctx.body.value;
  } else if (multipart) {
    if (adapter.jsonOnly)
      throw new Error("this model accepts JSON image requests only");
    req = {};
    for (const name of Object.keys(ctx.body.fields || {})) {
      const values = ctx.body.fields[name];
      if (!Array.isArray(values)) throw new Error("invalid multipart field");
      if (/^images?(\[\d*\])?$/.test(name)) {
        hasFormImages = true;
        for (const value of values) formImages.push(value);
      } else {
        if (values.length !== 1)
          throw new Error(name + " must be provided once");
        req[name] = values[0];
      }
    }
    for (const file of ctx.body.files || []) {
      if (/^image(\[\d*\])?$/.test(file.field)) uploads.push(file);
      else if (file.field === "mask" && !uploadedMask) uploadedMask = file;
      else throw new Error("unexpected or repeated file field: " + file.field);
    }
  } else throw new Error("JSON or multipart body required");

  let metadata = req.metadata;
  if (multipart && typeof metadata === "string") {
    try {
      metadata = JSON.parse(metadata);
    } catch (_) {
      throw new Error("metadata must be a JSON object");
    }
  }
  if (metadata !== undefined && !isObject(metadata))
    throw new Error("metadata must be an object");
  const suppliedParams = metadata && metadata.params;
  if (suppliedParams !== undefined && !isObject(suppliedParams))
    throw new Error("metadata.params must be an object");
  const params =
    suppliedParams === undefined ? {} : utils.json.clone(suppliedParams);
  if (
    req.response_format !== undefined &&
    req.response_format !== "url" &&
    req.response_format !== "b64_json"
  )
    throw new Error("response_format must be url or b64_json");
  if (req.prompt !== undefined && typeof req.prompt !== "string")
    throw new Error("prompt must be a string");

  const standard = {};
  for (const key of [
    "prompt",
    "size",
    "quality",
    "background",
    "output_format",
    "moderation",
    "user",
    "input_fidelity",
    "negative_prompt",
    "aspect_ratio",
    "resolution",
    "thinking_level",
  ]) {
    if (req[key] !== undefined) standard[key] = req[key];
  }
  for (const key of [
    "watermark",
    "prompt_extend",
    "thinking_mode",
    "layer_decomposition",
  ]) {
    if (req[key] === undefined) continue;
    let value = req[key];
    if (multipart && (value === "true" || value === "false"))
      value = value === "true";
    if (typeof value !== "boolean")
      throw new Error(key + " must be true or false");
    standard[key] = value;
  }
  if (req.seed !== undefined)
    standard.seed = boundedNumber(req.seed, "seed", 0, 4294967295, true);
  if (req.output_compression !== undefined)
    standard.output_compression = boundedNumber(
      req.output_compression,
      "output_compression",
      0,
      100,
      true,
    );
  const hasImages =
    req.image !== undefined ||
    req.images !== undefined ||
    hasFormImages ||
    uploads.length > 0;
  const images = [];
  const references = formImages.concat(
    req.image === undefined ? [] : req.image,
    req.images === undefined ? [] : req.images,
  );
  const bareBase64 = adapter.kind === "kirin" || adapter.kind === "flux";
  for (const value of references) {
    if (typeof value !== "string" || !trimmed(value))
      throw new Error("image must be an HTTP URL or Base64 data URL");
    const image = trimmed(value);
    if (
      !/^https?:\/\//i.test(image) &&
      !/^data:image\/[^;,]+;base64,/i.test(image)
    )
      throw new Error("image must be an HTTP URL or Base64 data URL");
    images.push(
      bareBase64 ? image.replace(/^data:[^;,]*;base64,/i, "") : image,
    );
  }
  for (const file of uploads) {
    images.push({
      __fileRef: file.ref,
      encoding: bareBase64 ? "base64" : "dataUrl",
      maxBytes: MAX_IMAGE_BYTES,
    });
  }
  if (uploadedMask && adapter.kind !== "gpt")
    throw new Error("mask files are not supported by this model");
  if (req.mask !== undefined && adapter.kind !== "gpt")
    throw new Error("mask is not supported by this model");
  if (req.mask !== undefined && uploadedMask)
    throw new Error("mask must be provided once");

  let imageCount = images.length;
  if (["messages", "wan25", "wan26"].indexOf(adapter.kind) >= 0) {
    if (params.input !== undefined && !isObject(params.input))
      throw new Error("metadata.params.input must be an object");
    if (params.parameters !== undefined && !isObject(params.parameters))
      throw new Error("metadata.params.parameters must be an object");
    const input = params.input || {};
    const options = params.parameters || {};
    params.input = input;
    params.parameters = options;
    for (const key of [
      "size",
      "watermark",
      "prompt_extend",
      "thinking_mode",
      "seed",
    ])
      if (standard[key] !== undefined) options[key] = standard[key];
    if (typeof options.size === "string") {
      if (options.size.toLowerCase() === "auto") {
        if (adapter.kind === "wan26") options.size = "1K";
        else delete options.size;
      } else if (/^\d+[x×]\d+$/i.test(options.size))
        options.size = options.size.replace(/[x×]/i, "*");
    }
    if (adapter.kind === "wan25") {
      for (const key of ["prompt", "negative_prompt"])
        if (standard[key] !== undefined) input[key] = standard[key];
      if (hasImages) input.images = images;
      imageCount =
        input.images === undefined ? 0 : [].concat(input.images).length;
      if (adapter.edit && (imageCount < 1 || imageCount > 3))
        throw new Error("editing requires between 1 and 3 input images");
      if (!adapter.edit && imageCount)
        throw new Error("this model supports text-to-image only");
      if (!trimmed(input.prompt)) throw new Error("prompt is required");
      if (req.n !== undefined)
        options.n = boundedNumber(req.n, "n", 1, 4, true);
      else if (options.n === undefined) options.n = 1;
    } else {
      if (input.messages !== undefined && !Array.isArray(input.messages))
        throw new Error("input.messages must be an array");
      if (
        (req.prompt !== undefined || hasImages) &&
        input.messages &&
        input.messages.length > 1
      )
        throw new Error("standard image fields require a single input message");
      if (req.prompt !== undefined || hasImages || !input.messages) {
        const message =
          input.messages && input.messages.length
            ? input.messages[0]
            : { role: "user" };
        if (!isObject(message))
          throw new Error("input.messages entries must be objects");
        if (message.content !== undefined && !Array.isArray(message.content))
          throw new Error("message content must be an array");
        let content = message.content || [];
        if (req.prompt !== undefined) {
          content = content.filter(
            (part) => !isObject(part) || part.text === undefined,
          );
          content.push({ text: req.prompt });
        }
        if (hasImages) {
          content = content.filter(
            (part) => !isObject(part) || part.image === undefined,
          );
          for (const image of images) content.push({ image: image });
        }
        message.content = content;
        if (message.role === undefined) message.role = "user";
        input.messages = [message];
      }
      let hasPrompt = false;
      imageCount = 0;
      for (const message of input.messages || []) {
        if (!isObject(message) || !Array.isArray(message.content))
          throw new Error("invalid image message");
        for (const part of message.content) {
          if (!isObject(part)) throw new Error("invalid image content entry");
          if (typeof part.text === "string" && part.text.trim())
            hasPrompt = true;
          if (part.image !== undefined) imageCount++;
        }
      }
      if (!hasPrompt) throw new Error("prompt is required");
      if (standard.negative_prompt !== undefined)
        options.negative_prompt = standard.negative_prompt;
      if (adapter.kind === "wan26") {
        if (options.enable_interleave === undefined)
          options.enable_interleave = ctx.operation !== "edit";
        if (typeof options.enable_interleave !== "boolean")
          throw new Error("enable_interleave must be true or false");
        if (options.enable_interleave) {
          if (imageCount > 1)
            throw new Error(
              "interleaved generation accepts at most one input image",
            );
          if (options.n === undefined) options.n = 1;
          if (req.n !== undefined)
            options.max_images = boundedNumber(req.n, "n", 1, 5, true);
          else if (options.max_images === undefined) options.max_images = 5;
        } else {
          if (imageCount < 1 || imageCount > 4)
            throw new Error("editing requires between 1 and 4 input images");
          if (req.n !== undefined)
            options.n = boundedNumber(req.n, "n", 1, 4, true);
          else if (options.n === undefined) options.n = 1;
        }
      } else {
        if (adapter.edit && !imageCount)
          throw new Error("image is required for editing");
        if (req.n !== undefined)
          options.n = boundedNumber(req.n, "n", 1, adapter.countMax, true);
        else if (options.n === undefined) options.n = 1;
      }
    }
  } else if (adapter.kind === "gpt") {
    if (ctx.operation === "edit" && !adapter.edit)
      throw new Error("select a GPT Image edit model for this endpoint");
    for (const key of [
      "prompt",
      "size",
      "quality",
      "background",
      "output_format",
      "output_compression",
      "moderation",
      "user",
      "input_fidelity",
    ])
      if (standard[key] !== undefined) params[key] = standard[key];
    if (hasImages) params.images = images;
    imageCount =
      params.images === undefined ? 0 : [].concat(params.images).length;
    if (adapter.edit && !imageCount)
      throw new Error("image is required for editing");
    if (!adapter.edit && imageCount)
      throw new Error(
        "select a GPT Image edit model when providing input images",
      );
    if (req.mask !== undefined) params.mask = utils.json.clone(req.mask);
    if (uploadedMask)
      params.mask = {
        __fileRef: uploadedMask.ref,
        encoding: "dataUrl",
        maxBytes: MAX_IMAGE_BYTES,
      };
    if (req.n !== undefined) params.n = boundedNumber(req.n, "n", 1, 4, true);
    else if (params.n === undefined) params.n = 1;
    if (!trimmed(params.prompt)) throw new Error("prompt is required");
  } else {
    for (const key of [
      "prompt",
      "negative_prompt",
      "seed",
      "watermark",
      "prompt_extend",
      "aspect_ratio",
      "resolution",
      "thinking_level",
      "output_format",
      "output_compression",
      "background",
      "layer_decomposition",
    ])
      if (standard[key] !== undefined) params[key] = standard[key];
    if (adapter.kind === "flux" || adapter.kind === "flux3") {
      if (req.n !== undefined) boundedNumber(req.n, "n", 1, 1, true);
      if (params.n !== undefined)
        boundedNumber(params.n, "metadata.params.n", 1, 1, true);
      if (hasImages) {
        if (images.length > adapter.maxImages)
          throw new Error("too many input images for this model");
        if (adapter.kind === "flux3") {
          delete params.image;
          params.images = images;
        } else {
          for (let i = 0; i < adapter.maxImages; i++)
            delete params[i ? "input_image_" + (i + 1) : "input_image"];
          for (let i = 0; i < images.length; i++)
            params[i ? "input_image_" + (i + 1) : "input_image"] = images[i];
        }
      }
      if (adapter.kind === "flux3")
        imageCount =
          params.images === undefined
            ? params.image === undefined
              ? 0
              : [].concat(params.image).length
            : [].concat(params.images).length;
      else {
        imageCount = 0;
        for (let i = 0; i < adapter.maxImages; i++)
          if (
            params[i ? "input_image_" + (i + 1) : "input_image"] !== undefined
          )
            imageCount++;
      }
      if (!adapter.maxImages && ctx.operation === "edit")
        throw new Error("this model supports generation only");
      if (
        standard.size !== undefined &&
        String(standard.size).toLowerCase() !== "auto"
      ) {
        if (adapter.kind === "flux3") params.size = standard.size;
        else {
          const dimensions = /^(\d+)[x*×](\d+)$/i.exec(String(standard.size));
          if (!dimensions) throw new Error("size must be WxH for this model");
          const width = boundedNumber(
            dimensions[1],
            "width",
            1,
            16777216,
            true,
          );
          const height = boundedNumber(
            dimensions[2],
            "height",
            1,
            16777216,
            true,
          );
          if (model.indexOf("flux_kontext_") === 0) {
            let a = width;
            let b = height;
            while (b) {
              const remainder = a % b;
              a = b;
              b = remainder;
            }
            params.aspect_ratio = width / a + ":" + height / a;
          } else {
            params.width = width;
            params.height = height;
          }
        }
      }
    } else if (adapter.kind === "nano") {
      if (req.n !== undefined) boundedNumber(req.n, "n", 1, 1, true);
      if (params.n !== undefined)
        boundedNumber(params.n, "metadata.params.n", 1, 1, true);
      if (
        isObject(params.generationConfig) &&
        params.generationConfig.candidateCount !== undefined
      )
        boundedNumber(
          params.generationConfig.candidateCount,
          "candidateCount",
          1,
          1,
          true,
        );
      if (
        params.contents !== undefined &&
        (req.prompt !== undefined || hasImages)
      )
        throw new Error(
          "native contents cannot be combined with standard prompt or image fields",
        );
      if (hasImages) params.image_urls = images;
      if (params.image_urls !== undefined) {
        if (!Array.isArray(params.image_urls))
          throw new Error("image_urls must be an array");
        for (const image of params.image_urls)
          if (typeof image !== "string" || !/^https?:\/\//i.test(image))
            throw new Error("Nano Banana image_urls require public HTTP URLs");
      }
      imageCount = params.image_urls ? params.image_urls.length : 0;
      if (params.contents !== undefined) {
        if (model !== "nano_banana_2_1" || !Array.isArray(params.contents))
          throw new Error("native contents are not supported for this model");
        for (const message of params.contents)
          for (const part of message.parts || [])
            if (isObject(part.inlineData)) imageCount++;
      }
      if (
        standard.size !== undefined &&
        String(standard.size).toLowerCase() !== "auto"
      ) {
        let resolution = String(standard.size);
        if (model !== "nano_banana") {
          const square = {
            "512x512": "0.5K",
            "1024x1024": "1K",
            "2048x2048": "2K",
            "4096x4096": "4K",
          };
          resolution = square[resolution] || resolution;
          if (!/^(?:0\.5|1|2|4)K$/.test(resolution))
            throw new Error(
              "use a documented resolution tier in metadata.params for this model",
            );
          if (model === "nano_banana_2_lite" && resolution !== "1K")
            throw new Error("this model supports 1K only");
          if (model !== "nano_banana_2" && resolution === "0.5K")
            throw new Error("0.5K is not supported by this model");
        }
        params[model === "nano_banana_2_lite" ? "image_size" : "resolution"] =
          resolution;
      }
      if (params.contents === undefined && !trimmed(params.prompt))
        throw new Error("prompt is required");
    } else if (adapter.kind === "kirin") {
      if (hasImages) {
        if (images.length > adapter.maxImages)
          throw new Error("too many input images for this model");
        if (adapter.omni)
          params.image_list = images.map((image) => ({ image: image }));
        else if (images.length) params.image = images[0];
        else delete params.image;
      }
      imageCount = adapter.omni
        ? (params.image_list || []).length
        : params.image === undefined
          ? 0
          : 1;
      if (
        standard.size !== undefined &&
        String(standard.size).toLowerCase() !== "auto"
      )
        throw new Error(
          "use metadata.params.aspect_ratio and resolution for this model",
        );
      if (req.n !== undefined) {
        if (model === "kirin_v3_omni_image" && params.result_type === "series")
          params.series_amount = boundedNumber(req.n, "n", 2, 9, true);
        else params.n = boundedNumber(req.n, "n", 1, 9, true);
      } else if (params.n === undefined && params.result_type !== "series")
        params.n = 1;
      if (!trimmed(params.prompt)) throw new Error("prompt is required");
    } else if (adapter.kind === "spark") {
      if (hasImages) params.image = images;
      imageCount =
        params.image === undefined ? 0 : [].concat(params.image).length;
      if (imageCount > adapter.maxImages)
        throw new Error("too many input images for this model");
      if (standard.size !== undefined) {
        if (
          String(standard.size).toLowerCase() === "auto" &&
          params.layer_decomposition !== true
        )
          delete params.size;
        else params.size = standard.size;
      }
      if (adapter.layers) {
        if (req.n !== undefined) boundedNumber(req.n, "n", 1, 1, true);
        if (params.layer_decomposition === true && imageCount !== 1)
          throw new Error(
            "layer decomposition requires exactly one input image",
          );
      } else {
        if (req.n !== undefined) {
          const n = boundedNumber(req.n, "n", 1, 15, true);
          if (
            params.sequential_image_generation !== undefined &&
            params.sequential_image_generation !== "auto" &&
            n > 1
          )
            throw new Error(
              "multiple outputs require sequential_image_generation auto",
            );
          if (n > 1 || params.sequential_image_generation === "auto") {
            params.sequential_image_generation = "auto";
            if (
              params.sequential_image_generation_options !== undefined &&
              !isObject(params.sequential_image_generation_options)
            )
              throw new Error(
                "sequential_image_generation_options must be an object",
              );
            if (!params.sequential_image_generation_options)
              params.sequential_image_generation_options = {};
            params.sequential_image_generation_options.max_images = n;
          }
        }
        if (params.sequential_image_generation === "auto") {
          if (
            params.sequential_image_generation_options !== undefined &&
            !isObject(params.sequential_image_generation_options)
          )
            throw new Error(
              "sequential_image_generation_options must be an object",
            );
          if (!params.sequential_image_generation_options)
            params.sequential_image_generation_options = {};
          if (
            params.sequential_image_generation_options.max_images === undefined
          )
            params.sequential_image_generation_options.max_images =
              15 - imageCount;
        }
      }
      if (params.layer_decomposition !== true && !trimmed(params.prompt))
        throw new Error("prompt is required");
    } else if (adapter.kind === "grok") {
      if (ctx.operation === "edit" && !adapter.edit)
        throw new Error("select a Grok Image edit model for this endpoint");
      if (hasImages) {
        delete params.image;
        params.images = images.map((url) => ({ url: url, type: "image_url" }));
      }
      imageCount =
        params.images === undefined
          ? params.image === undefined
            ? 0
            : 1
          : [].concat(params.images).length;
      if (adapter.edit && !imageCount)
        throw new Error("image is required for editing");
      if (!adapter.edit && imageCount)
        throw new Error(
          "select a Grok Image edit model when providing input images",
        );
      if (req.n !== undefined)
        params.n = boundedNumber(req.n, "n", 1, MAX_IMAGE_OUTPUTS, true);
      else if (params.n === undefined) params.n = 1;
      if (!trimmed(params.prompt)) throw new Error("prompt is required");
    }
    if (
      (adapter.kind === "flux" || adapter.kind === "flux3") &&
      !trimmed(params.prompt)
    )
      throw new Error("prompt is required");
  }
  if (ctx.operation === "edit" && !imageCount)
    throw new Error("image is required for editing");
  const body = { model: ctx.model, input: [{ params: params }] };
  validateNativeGeneration(body, model);
  return body;
}

// Flat video request shared by the OpenAI video protocol and the native route.
// wan27_i2v generates from frames, so at least one first_frame media entry is
// required; the remaining vendor media kinds pass through unchanged.
function validateVideoRequest(req) {
  const prompt = trimmed(req.prompt);
  const media = [];
  for (const entry of req.media || []) {
    if (!entry || typeof entry !== "object")
      throw new Error("media entries must be objects");
    const type = trimmed(entry.type);
    if (VIDEO_MEDIA_TYPES.indexOf(type) < 0)
      throw new Error(
        "media type must be one of " + VIDEO_MEDIA_TYPES.join(", "),
      );
    let url = entry.url;
    if (url && typeof url === "object" && url.__fileRef)
      url = filePlaceholder(url.__fileRef);
    if (!isImageInput(url) || typeof url !== "string") {
      if (!(url && typeof url === "object" && url.__fileRef))
        throw new Error("media url must be an HTTP URL or a Base64 data URL");
    }
    media.push({ type: type, url: url });
  }
  const hasFilePlaceholder = media.some(function (entry) {
    return entry.url && typeof entry.url === "object" && entry.url.__fileRef;
  });
  if (!prompt && !media.length)
    throw new Error("prompt or a first_frame media entry is required");
  const firstFrame = media.filter(function (entry) {
    return entry.type === "first_frame";
  });
  if (!firstFrame.length && !hasFilePlaceholder && !prompt)
    throw new Error("wan27_i2v requires a first_frame media entry");
  if (!firstFrame.length && !hasFilePlaceholder)
    throw new Error("wan27_i2v requires a first_frame media entry");
  const duration = optionalInt(
    req.duration === undefined ? 5 : req.duration,
    "duration",
    1,
    MAX_DURATION_SECONDS,
  );
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
    if (audioSetting !== "auto" && audioSetting !== "origin")
      throw new Error("audio_setting must be auto or origin");
    flat.audio_setting = audioSetting;
  }
  return flat;
}

function videoRequestFromOpenAI(ctx) {
  let req;
  let inputReferenceFileRef;
  if (ctx.body && ctx.body.kind === "json") {
    req = utils.json.clone(ctx.body.value);
    if (!req || typeof req !== "object" || Array.isArray(req))
      throw new Error("request body must be an object");
  } else if (ctx.body && ctx.body.kind === "multipart") {
    const fields = ctx.body.fields || {};
    req = {};
    for (const name of Object.keys(fields)) {
      const values = fields[name];
      if (name !== "input_reference" && values.length > 1)
        throw new Error(name + " must be provided once");
      req[name] = values[0];
    }
    for (const file of ctx.body.files || []) {
      if (file.field !== "input_reference")
        throw new Error("unexpected file field: " + file.field);
      if (inputReferenceFileRef)
        throw new Error("input_reference must be provided once");
      inputReferenceFileRef = file.ref;
    }
    if (req.metadata !== undefined) {
      let parsed;
      try {
        parsed = JSON.parse(req.metadata);
      } catch (e) {
        throw new Error("metadata must be a JSON object string");
      }
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
        throw new Error("metadata must be a JSON object string");
      req = Object.assign({}, parsed, req);
      delete req.metadata;
    }
  } else throw new Error("JSON or multipart body required");
  if (req.seconds !== undefined) req.duration = Number(req.seconds);
  if (req.size !== undefined && req.resolution === undefined)
    req.resolution = req.size;
  const media = [];
  if (inputReferenceFileRef)
    media.push({
      type: "first_frame",
      url: { __fileRef: inputReferenceFileRef },
    });
  else if (trimmed(req.input_reference))
    media.push({ type: "first_frame", url: trimmed(req.input_reference) });
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

// 3D generation runs only on the native route, so the client sends the Vtrix
// params object directly; validation mirrors the vendor field rules.
function threedParamsFromVtrix(params) {
  if (!params || typeof params !== "object" || Array.isArray(params))
    throw new Error("params must be an object");
  const generateType =
    params.generate_type === undefined ? "Normal" : params.generate_type;
  if (THREED_GENERATE_TYPES.indexOf(generateType) < 0)
    throw new Error(
      "generate_type must be one of " + THREED_GENERATE_TYPES.join(", "),
    );
  const prompt = trimmed(params.prompt);
  const imageUrl = trimmed(params.image_url);
  const imageBase64 = trimmed(params.image_base64);
  const inputs = [prompt, imageUrl, imageBase64].filter(function (value) {
    return value;
  }).length;
  if (generateType === "Sketch") {
    if (!prompt || (!imageUrl && !imageBase64))
      throw new Error("Sketch mode requires a prompt and a reference image");
    if (params.multi_view_images !== undefined)
      throw new Error("Sketch mode does not support multi_view_images");
  } else if (inputs !== 1) {
    throw new Error(
      generateType +
        " mode requires exactly one of prompt, image_url, or image_base64",
    );
  }
  if (prompt.length > MAX_THREED_PROMPT_RUNES)
    throw new Error(
      "prompt must be at most " + MAX_THREED_PROMPT_RUNES + " characters",
    );
  const flat = { generate_type: generateType };
  if (prompt) flat.prompt = prompt;
  if (imageUrl) {
    if (!/^https?:\/\//i.test(imageUrl))
      throw new Error("image_url must be an HTTP URL");
    flat.image_url = imageUrl;
  }
  if (imageBase64) flat.image_base64 = imageBase64;
  const faceCount = optionalInt(
    params.face_count,
    "face_count",
    MIN_FACE_COUNT,
    MAX_FACE_COUNT,
  );
  if (faceCount !== undefined) flat.face_count = faceCount;
  if (params.polygon_type !== undefined) {
    if (THREED_POLYGON_TYPES.indexOf(params.polygon_type) < 0)
      throw new Error("polygon_type must be triangle or quadrilateral");
    flat.polygon_type = params.polygon_type;
  }
  const enablePbr = optionalBool(params.enable_pbr, "enable_pbr");
  if (enablePbr !== undefined) flat.enable_pbr = enablePbr;
  if (params.multi_view_images !== undefined) {
    if (!Array.isArray(params.multi_view_images))
      throw new Error("multi_view_images must be an array");
    const views = [];
    for (const view of params.multi_view_images) {
      if (!view || typeof view !== "object")
        throw new Error("multi_view_images entries must be objects");
      if (THREED_VIEW_TYPES.indexOf(view.ViewType) < 0)
        throw new Error(
          "multi_view_images ViewType must be one of " +
            THREED_VIEW_TYPES.join(", "),
        );
      const viewUrl = trimmed(view.ViewImageUrl);
      const viewBase64 = trimmed(view.ViewImageBase64);
      if (!viewUrl && !viewBase64)
        throw new Error(
          "multi_view_images entries need ViewImageUrl or ViewImageBase64",
        );
      const copy = { ViewType: view.ViewType };
      if (viewUrl) {
        if (!/^https?:\/\//i.test(viewUrl))
          throw new Error("ViewImageUrl must be an HTTP URL");
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
  return validateNativeGeneration(req, model);
}

function vtrixImageBody(req) {
  const content = [];
  for (const image of req.images || []) content.push({ image: image });
  content.push({ text: req.prompt });
  const parameters = {};
  if (req.interleave) {
    parameters.enable_interleave = true;
    parameters.n = 1;
    if (req.max_images !== undefined && req.max_images !== 5)
      parameters.max_images = req.max_images;
  } else {
    parameters.n = req.n === undefined ? 1 : req.n;
  }
  if (req.size) parameters.size = req.size;
  if (req.negative_prompt) parameters.negative_prompt = req.negative_prompt;
  if (req.watermark !== undefined) parameters.watermark = req.watermark;
  if (req.seed !== undefined) parameters.seed = req.seed;
  if (!req.interleave && req.prompt_extend !== undefined)
    parameters.prompt_extend = req.prompt_extend;
  return {
    model: req.model,
    input: [
      {
        params: {
          input: { messages: [{ role: "user", content: content }] },
          parameters: parameters,
        },
      },
    ],
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
  if (req.prompt_extend !== undefined)
    parameters.prompt_extend = req.prompt_extend;
  if (req.audio_setting) parameters.audio_setting = req.audio_setting;
  return {
    model: req.model,
    input: [{ params: { input: input, parameters: parameters } }],
  };
}

function vtrixThreedBody(req) {
  return { model: req.model, input: [{ params: req.params }] };
}

function modelOf(ctx) {
  return (
    ctx.upstreamModel || (ctx.requestBody && ctx.requestBody.model) || ctx.model
  );
}

function authHeader(ctx) {
  return ctx.authHeader || "Bearer " + ctx.apiKey;
}

export function buildSubmitRequest(ctx) {
  const req = ctx.requestBody || {};
  const model = modelOf(ctx);
  let body;
  if (req.input !== undefined) body = validateNativeGeneration(req, model);
  else if (model === "wan26_image")
    body = vtrixImageBody(Object.assign({}, req, { model: model }));
  else if (model === "wan27_i2v")
    body = vtrixVideoBody(Object.assign({}, req, { model: model }));
  else if (model === "tencent_hunyuan_3d_pro")
    body = vtrixThreedBody(Object.assign({}, req, { model: model }));
  else throw new Error("unsupported model: " + String(model));
  return {
    url: generationURL(ctx),
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
  const message = trimmed(error.message) || trimmed(error.error_message);
  if (error.code || message)
    throw new Error((error.code || "submit_failed") + ": " + message);
  const taskId = trimmed(body.id);
  if (!taskId) throw new Error("missing generation id");
  const req = ctx.requestBody || {};
  let state;
  if (req.input !== undefined) {
    const model = modelOf(ctx);
    const facts = nativeUsage(validateNativeGeneration(req, model), model);
    state = { kind: modelKind(model), usage: facts };
  } else if (req.params) state = { kind: "threed" };
  else if (req.media) state = { kind: "video" };
  else state = { kind: "image", imageCount: imageCountFor(req) };
  const result = { taskId: taskId, taskData: body, state: state };
  if (body.status === "failed")
    result.immediate = {
      status: "FAILURE",
      reason: trimmed(error.message) || "task failed",
    };
  else if (body.status === "completed" && outputContent(body).length)
    result.immediate = parseTaskResult(ctx, body);
  return result;
}

export function buildQueryRequest(ctx) {
  return {
    url: generationURL(ctx) + "/task/" + encodeURIComponent(ctx.taskId),
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
      if (
        part &&
        typeof part === "object" &&
        typeof part.url === "string" &&
        part.url.trim()
      )
        entries.push(part);
    }
  }
  return entries;
}

function contentUrl(entries, type) {
  for (const entry of entries) {
    if ((!type || entry.type === type) && trimmed(entry.url))
      return trimmed(entry.url);
  }
  return "";
}

function isOutputUrl(value, type) {
  if (typeof value !== "string") return false;
  const url = value.trim();
  if (/^https?:\/\/[^/\s?#]+(?:[/?#]\S*)?$/i.test(url)) return true;
  if (type !== "image") return false;
  const data = /^data:image\/[\w.+-]+;base64,([A-Za-z0-9+/]+={0,2})$/i.exec(
    url,
  );
  return Boolean(data && data[1].length % 4 === 0);
}

export function parseTaskResult(ctx, body) {
  const status = trimmed(body && body.status);
  const model = modelOf(ctx);
  if (
    ["in_progress", "created", "queued", "in_queue", "pending"].indexOf(
      status,
    ) >= 0
  )
    return { status: "IN_PROGRESS" };
  if (status === "completed") {
    const entries = outputContent(body);
    const kind = modelKind(model);
    if (kind === "image") {
      const images = entries.filter(function (entry) {
        return entry.type === "image";
      });
      if (
        !images.length ||
        images.some(function (image) {
          return !isOutputUrl(image.url, "image");
        })
      )
        return {
          status: "FAILURE",
          reason: "image task completed without valid images",
        };
      return { status: "SUCCESS", url: trimmed(images[0].url) };
    }
    if (kind === "video" || kind === "voice") {
      const type = kind === "voice" ? "audio" : "video";
      const media = entries.filter(function (entry) {
        return entry.type === type;
      });
      const url = contentUrl(entries, type);
      if (
        !url ||
        media.some(function (entry) {
          return !isOutputUrl(entry.url, type);
        })
      )
        return {
          status: "FAILURE",
          reason: type + " task completed without valid " + type,
        };
      return { status: "SUCCESS", url: url };
    }
    const assets = entries.filter(function (entry) {
      return (
        artifactType(entry) === "file" ||
        (model === "hi3d_image_to_relief" && artifactType(entry) === "image")
      );
    });
    const url = assets.length ? trimmed(assets[0].url) : "";
    if (
      !url ||
      assets.some(function (asset) {
        return !isOutputUrl(asset.url, artifactType(asset));
      })
    )
      return {
        status: "FAILURE",
        reason: "3D task completed without a valid model file",
      };
    return { status: "SUCCESS", url: url };
  }
  if (status === "failed") {
    const error = (body && body.error) || {};
    const reason =
      trimmed(error.message) ||
      trimmed(error.error_message) ||
      trimmed(error.code) ||
      trimmed(body.message) ||
      "task failed";
    return { status: "FAILURE", reason: reason };
  }
  return { status: "UNKNOWN", reason: "unrecognized status: " + status };
}

export function extractUsage(ctx) {
  const req = ctx.requestBody || {};
  if (req.input !== undefined) {
    const model = modelOf(ctx);
    const facts = nativeUsage(validateNativeGeneration(req, model), model);
    if (ctx.usagePurpose === "billing_ratios" && model === "wan27_i2v")
      return { seconds: facts.duration };
    return facts;
  }
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
  if (
    req.kind !== "image" ||
    (req.usage && req.usage.image_count === undefined)
  )
    return {};
  const entries = outputContent(body);
  const payloadCount = entries.filter(function (entry) {
    return entry.type === "image";
  }).length;
  const usage = (body && body.usage) || {};
  const extra = usage.extra_info || {};
  for (const value of [
    usage.generated_images,
    usage.image_count,
    extra.num_images,
  ]) {
    if (value === undefined || value === null) continue;
    const reported = Number(value);
    if (
      !Number.isInteger(reported) ||
      reported < 1 ||
      reported > MAX_IMAGE_OUTPUTS ||
      (payloadCount && reported !== payloadCount)
    )
      throw new Error("invalid upstream image count");
    return { image_count: reported };
  }
  if (payloadCount > MAX_IMAGE_OUTPUTS)
    throw new Error("too many output images");
  if (payloadCount) return { image_count: payloadCount };
  return {};
}

function artifactType(entry) {
  const format = trimmed(entry.format).toLowerCase();
  if (["png", "jpg", "jpeg", "bmp", "webp", "exr"].indexOf(format) >= 0)
    return "image";
  if (["image", "video", "audio"].indexOf(entry.type) >= 0) return entry.type;
  const types = [
    "model3d",
    "glb",
    "gltf",
    "obj",
    "stl",
    "fbx",
    "usdz",
    "3mf",
    "file",
    "zip",
  ];
  return types.indexOf(entry.type) >= 0 || types.indexOf(format) >= 0
    ? "file"
    : "";
}

export function listArtifacts(task) {
  if (!task || task.status !== "SUCCESS") return [];
  const counts = {};
  const artifacts = [];
  for (const entry of outputContent(task.data)) {
    const type = artifactType(entry);
    if (!type || !/^https?:\/\//i.test(entry.url)) continue;
    counts[type] = (counts[type] || 0) + 1;
    const key =
      type === "video" && counts[type] === 1
        ? "video"
        : type + "-" + counts[type];
    artifacts.push({ key: key, type: type });
  }
  return artifacts;
}

export function buildContentRequest(ctx) {
  const data = (ctx && ctx.data) || {};
  const artifacts = listArtifacts({ status: "SUCCESS", data: data });
  const index = artifacts.findIndex(function (artifact) {
    return artifact.key === ctx.artifactKey;
  });
  const entries = outputContent(data).filter(function (entry) {
    return artifactType(entry) !== "" && /^https?:\/\//i.test(entry.url);
  });
  const url = index >= 0 ? trimmed(entries[index].url) : "";
  if (!url) throw new Error("artifact_not_found");
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}

export const native = {
  submitGeneration: function (ctx) {
    const req = requireJsonObject(ctx);
    const flat = decodeGenerationBody(req);
    const actions = {
      image: "generate_image",
      video: "generate_video",
      model: "generate_3d_asset",
      voice: "generate_audio",
    };
    const action = actions[modelKind(flat.model)];
    return {
      kind: "submit",
      model: flat.model,
      action: action,
      requestBody: flat,
    };
  },
  generationCreated: function (ctx, task) {
    const data = task.data || {};
    return {
      id: task.task_id,
      created_at: data.created_at !== undefined ? data.created_at : Date.now(),
      status:
        task.status === "SUCCESS"
          ? "completed"
          : task.status === "FAILURE"
            ? "failed"
            : "in_progress",
      error: null,
      model: ctx.model,
    };
  },
  generationStatus: function (ctx, task) {
    return (
      task.data || { id: task.task_id, status: "in_progress", error: null }
    );
  },
  error: function (ctx, error) {
    return { code: error.code, message: error.message };
  },
};

export const protocols = {
  openai_image: {
    decodeRequest: function (ctx) {
      return {
        kind: "submit",
        model: ctx.model,
        action: ctx.operation === "edit" ? "image_edit" : "generate_image",
        requestBody: imageRequestFromOpenAI(ctx),
      };
    },
    render: function (ctx, task) {
      const data = [];
      for (const entry of outputContent(task.data)) {
        if (entry.type !== "image") continue;
        const url = trimmed(entry.url);
        if (/^https?:\/\//i.test(url)) data.push({ url: url });
        else if (/^data:[^;,]*;base64,(.*)$/i.test(url))
          data.push({ b64_json: url.replace(/^data:[^;,]*;base64,/i, "") });
        else throw new Error("invalid upstream image URL");
      }
      return { created: task.created_at, data: data };
    },
  },
  openai_video: {
    decodeRequest: function (ctx) {
      return {
        kind: "submit",
        model: ctx.model,
        action: "image_to_video",
        requestBody: videoRequestFromOpenAI(ctx),
      };
    },
    render: function (ctx, task) {
      const data = task.data || {};
      const statuses = {
        in_progress: "in_progress",
        completed: "completed",
        failed: "failed",
      };
      const rawProgress = Number(data.progress);
      const progress =
        Number.isFinite(rawProgress) && rawProgress >= 0 && rawProgress <= 1
          ? Math.round(rawProgress * 100)
          : 0;
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
        output.error = {
          code: data.error.code || "",
          message: data.error.message || "",
        };
      return output;
    },
  },
};
