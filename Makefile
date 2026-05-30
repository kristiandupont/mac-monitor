.PHONY: build run dev deps web-build icons app clean

BUNDLE     = Mac Monitor.app
BUNDLE_ID  = com.kristiandupont.mac-monitor

deps:
	go mod tidy
	cd web && npm install

web-build:
	cd web && npm run build

# Copy built web assets into the embed package, then build the Go binary.
build: deps web-build
	rm -rf internal/webui/dist
	cp -r web/dist internal/webui/dist
	go build -o mac-monitor ./cmd/mac-monitor

run: build
	./mac-monitor

# Start Go server + Vite dev server side-by-side.
# Browse http://localhost:5173 for the frontend (Vite proxies /api to :8080).
dev: web-build
	rm -rf internal/webui/dist
	cp -r web/dist internal/webui/dist
	go run ./cmd/mac-monitor &
	cd web && npm run dev

# Generate AppIcon.icns from the fan SVG (requires librsvg: brew install librsvg).
icons:
	bash build/generate-icons.sh

# Package everything into a self-contained .app bundle.
app: build icons
	rm -rf $(BUNDLE)
	mkdir -p "$(BUNDLE)/Contents/MacOS"
	mkdir -p "$(BUNDLE)/Contents/Resources"
	cp mac-monitor             "$(BUNDLE)/Contents/MacOS/"
	cp build/Info.plist        "$(BUNDLE)/Contents/"
	cp build/AppIcon.icns      "$(BUNDLE)/Contents/Resources/"
	cp build/PrivacyInfo.xcprivacy "$(BUNDLE)/Contents/Resources/"
	@echo "Built $(BUNDLE)"
	@echo ""
	@echo "To sign for distribution (requires Apple Developer account):"
	@echo "  codesign --deep --force --options runtime \\"
	@echo "    --entitlements build/mac-monitor.entitlements \\"
	@echo "    --sign 'Developer ID Application: Your Name (TEAMID)' \\"
	@echo "    $(BUNDLE)"

clean:
	rm -rf mac-monitor $(BUNDLE) internal/webui/dist build/AppIcon.icns build/AppIcon.iconset build/AppIcon.svg
