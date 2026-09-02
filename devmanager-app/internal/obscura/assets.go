package obscura

import (
	"fmt"
	"runtime"
)

// Versión pinnada del binario Obscura. No hay update-checker: YAGNI.
const Version = "v0.2.1"

const releaseBaseURL = "https://github.com/h4ckf0r0day/obscura/releases/download/" + Version

// assetSpec describe un artefacto de release por plataforma.
type assetSpec struct {
	FileName string // nombre del archivo en el release
	SHA256   string // digest sha256 del archivo (de GitHub API)
	Archive  string // "zip" | "targz"
	Binary   string // nombre del binario dentro del archivo
}

// assetForPlatform resuelve el asset para goos/goarch. Soportadas las 4
// combinaciones con release binario (con render, no no-render/stealth).
func assetForPlatform(goos, goarch string) (assetSpec, error) {
	switch goos {
	case "windows":
		if goarch != "amd64" {
			return assetSpec{}, fmt.Errorf("unsupported arch %q for obscura on windows", goarch)
		}
		return assetSpec{
			FileName: "obscura-x86_64-windows.zip",
			SHA256:   "202e7705c30b00026dcc3d493e1d5ef4ffb436767aaf84baaec11c7ff15a1a09",
			Archive:  "zip",
			Binary:   "obscura.exe",
		}, nil
	case "linux":
		switch goarch {
		case "amd64":
			return assetSpec{
				FileName: "obscura-x86_64-linux.tar.gz",
				SHA256:   "6a1a66b3f1ab118fa7d31330894a868617aea68c06d75436d851356c39df1ed3",
				Archive:  "targz",
				Binary:   "obscura",
			}, nil
		case "arm64":
			return assetSpec{
				FileName: "obscura-aarch64-linux.tar.gz",
				SHA256:   "0297c26d583f598f0126a7271cc40750598a9a9cbd86d1d6f79b2b99097d5244",
				Archive:  "targz",
				Binary:   "obscura",
			}, nil
		default:
			return assetSpec{}, fmt.Errorf("unsupported arch %q for obscura on linux", goarch)
		}
	case "darwin":
		arch := "x86_64"
		sha := "e6d0f8719998fa4460bccc712b20a1e524717d5c54e943f345227bd893ec9620"
		switch goarch {
		case "amd64":
		case "arm64":
			arch = "aarch64"
			sha = "5233da6426ec16667d7e4374b824189c6dfb3b325e5cf3fb5f04c7bc48b52a0f"
		default:
			return assetSpec{}, fmt.Errorf("unsupported arch %q for obscura on darwin", goarch)
		}
		return assetSpec{
			FileName: "obscura-" + arch + "-macos.tar.gz",
			SHA256:   sha,
			Archive:  "targz",
			Binary:   "obscura",
		}, nil
	}
	return assetSpec{}, fmt.Errorf("unsupported os %q for obscura", goos)
}

// currentAsset resuelve el asset de la plataforma actual.
func currentAsset() (assetSpec, error) {
	return assetForPlatform(runtime.GOOS, runtime.GOARCH)
}