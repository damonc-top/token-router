/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// VChart 2.x 不再自动注册浏览器渲染环境：若在 `new VChart()` 之前未显式注册，
// `vglobal.envContribution` 为 undefined，渲染图表时 `createCanvas` 会抛
// "Cannot read properties of undefined (reading 'createCanvas')"，导致控制台白屏。
// 这里在模块加载时同步注册浏览器环境并强制激活，确保后续图表实例化时画布已就绪。
import { registerBrowserEnv, vglobal } from '@visactor/vchart'

registerBrowserEnv()
vglobal.setEnv('browser', { force: true })

export const VCHART_OPTION = {
  // 与老前端保持一致（浏览器环境渲染优化）
  mode: 'desktop-browser',
} as const
