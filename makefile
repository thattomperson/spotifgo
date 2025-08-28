# Run templ generation in watch mode
watch-templ:
	go tool templ generate --watch --proxy="http://localhost:8090" --open-browser=false

# Run air for Go hot reload
server:
	go tool air \
	--build.cmd "go build -o tmp/bin/main ." \
	--build.bin "tmp/bin/main" \
	--build.delay "100" \
	--build.exclude_dir "node_modules" \
	--build.include_ext "go" \
	--build.stop_on_error "false" \
	--misc.clean_on_exit true

# Watch Tailwind CSS changes
watch-css: $(TAILWIND)
	$(TAILWIND) -i ./assets/css/input.css -o ./assets/css/output.css --watch

assets/css/output.css: assets/css/input.css $(TAILWIND)
	$(TAILWIND) -i $< -o $@;


# Start development server with all watchers
dev:
	make -j3 watch-css watch-templ watch-server


