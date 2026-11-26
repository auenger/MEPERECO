# MEPER Studio · 人机协作宣传片

基于原 `/Users/ryan/mycode/silieco/promo` 的 V3 协作生态版重新制作，保留原有 76 秒剧情、头像、工作 SOP 演绎、背景音乐及提示音。原工程未修改。品牌文字、应用图标及横版 Logo 采用当前仓库 `MEPER-Studio-Logo` 的素材；横版 SVG 仅修正画布宽度，避免字标末尾被裁切。

## 无头渲染

在此目录执行（Node.js 22+，依赖锁定）:

```sh
npm ci
npm run music
npm run render -- --browser-executable="/path/to/Chrome"
npm run captions
npm run check
```

本次在 macOS 使用已安装的 Google Chrome，以 headless 模式及 ANGLE 渲染；未打开可见浏览器。`out/v3/MEPER-Studio-人机协作宣传片.mp4` 为 H.264/AAC 主片（1920×1080、30fps、2280 帧，背景音乐和提示音，无真人配音）。`out/v3/讲解字幕.vtt` 是 Web 外挂字幕，另有 SRT、JSON、配音文案和时间轴。封面可用终场 2220 帧生成。`scripts/finish-v3.py` 可选生成无音轨版和故事板，需要 Pillow 与 FFmpeg。

首页播放资产位于 `../apps/web/public/media/meper-studio-collaboration-v3.*`。源码、头像、Logo、音频生成器与锁文件一并保留；`node_modules`、生成的 WAV 和 `out` 不纳入版本管理。
