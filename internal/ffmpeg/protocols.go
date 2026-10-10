package ffmpeg

import (
	"context"
	"fmt"
	"strings"
)

// ValidateNetworkProtocols checks the capabilities required for downloading
// public HTTPS media and HLS/DASH fragments. FFmpeg reports protocols as
// whitespace-separated names under Input/Output sections.
func ValidateNetworkProtocols(output string) error {
	protocols := make(map[string]struct{})
	for _, token := range strings.Fields(strings.ToLower(output)) {
		protocols[token] = struct{}{}
	}
	for _, required := range []string{"https", "tls"} {
		if _, ok := protocols[required]; !ok {
			return fmt.Errorf("ffmpeg: bundled binary lacks %s protocol support", required)
		}
	}
	return nil
}

// VerifyNetworkProtocols runs the bundled binary and checks that it can open
// HTTPS/TLS inputs before a download task starts.
func (inv *Invoker) VerifyNetworkProtocols(ctx context.Context) error {
	res, err := inv.RunFFmpeg(ctx, []string{"-protocols"})
	if err != nil {
		return err
	}
	return ValidateNetworkProtocols(string(res.Stdout) + "\n" + string(res.Stderr))
}
