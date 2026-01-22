build-image:
	podman buildx build --platform linux/amd64,linux/arm64 -f Dockerfile -t config-reloader-sidecar:latest .
