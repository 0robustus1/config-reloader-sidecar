build-image:
	docker buildx build --platform linux/amd64,linux/arm64 -t config-reloader-sidecar:latest .
