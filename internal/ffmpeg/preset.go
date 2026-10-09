package ffmpeg

import (
	"videodl/internal/media"
)

type Preset struct {
	Profile         media.DownloadProfile
	Name            string
	Description     string
	OutputExtension string
	VideoCodec      string
	AudioCodec      string
	StreamCopy      bool
	Args            []string
	QualityLossHint string
	TimeCostHint    string
}

var presets = map[media.DownloadProfile]Preset{
	media.ProfileOriginal: {
		Profile:         media.ProfileOriginal,
		Name:            "原始质量",
		Description:     "无损封装，直接复制流数据，不进行重编码",
		OutputExtension: ".mp4",
		VideoCodec:      "copy",
		AudioCodec:      "copy",
		StreamCopy:      true,
		Args: []string{
			"-c", "copy",
		},
		QualityLossHint: "无质量损失",
		TimeCostHint:    "极快（仅封装）",
	},
	media.ProfileMP4: {
		Profile:         media.ProfileMP4,
		Name:            "兼容 MP4 (H.264/AAC)",
		Description:     "通用 MP4 格式，适用于大多数播放器和移动设备",
		OutputExtension: ".mp4",
		VideoCodec:      "libx264",
		AudioCodec:      "aac",
		StreamCopy:      false,
		Args: []string{
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-crf", "23",
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
		},
		QualityLossHint: "有质量损失（CRF 23 均衡质量）",
		TimeCostHint:    "较慢（重编码耗时约为时长的 1-3 倍，取决于源文件复杂度和 CPU）",
	},
}

func GetPreset(profile media.DownloadProfile) (Preset, bool) {
	if profile == "" {
		profile = media.ProfileOriginal
	}
	p, ok := presets[profile]
	return p, ok
}

func AllPresets() []Preset {
	result := make([]Preset, 0, len(presets))
	for _, p := range presets {
		result = append(result, p)
	}
	return result
}
