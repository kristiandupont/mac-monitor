.PHONY: build run dev deps web-build lock icons app pkg clean

BUNDLE     = Mac Monitor.app
BUNDLE_ID  = com.kristiandupont.mac-monitor
TEAM_ID   ?= $(shell security find-identity -v -p codesigning 2>/dev/null | grep "3rd Party Mac Developer Application" | sed 's/.*(\(.*\))/\1/' | head -1)

deps:
	go mod tidy
	cd web && npm install

# Regenerate web/package-lock.json on Linux so CI gets all platform-specific
# optional deps (e.g. @emnapi/core) that macOS npm omits. Run after any npm install.
lock:
	docker run --rm -v "$(CURDIR)/web":/app -w /app node:24 npm install

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

# Build a signed .pkg for Mac App Store submission.
# Requires "3rd Party Mac Developer Application" and "3rd Party Mac Developer Installer"
# certificates installed in Keychain, and a Mac App Store provisioning profile at
# build/mac-monitor.provisionprofile (downloaded from developer.apple.com).
pkg: app
	@if [ -z "$(TEAM_ID)" ]; then \
	  echo "ERROR: Could not detect team ID. Run: make pkg TEAM_ID=XXXXXXXXXX"; exit 1; fi
	cp build/mac-monitor.provisionprofile "$(BUNDLE)/Contents/embedded.provisionprofile"
	codesign --deep --force --options runtime \
	  --entitlements build/mac-monitor.entitlements \
	  --sign "3rd Party Mac Developer Application: Kristian Dupont ($(TEAM_ID))" \
	  "$(BUNDLE)"
	productbuild --component "$(BUNDLE)" /Applications \
	  --sign "3rd Party Mac Developer Installer: Kristian Dupont ($(TEAM_ID))" \
	  mac-monitor.pkg
	@echo "Built mac-monitor.pkg — upload via Transporter or: xcrun altool --upload-app -f mac-monitor.pkg"

clean:
	rm -rf mac-monitor mac-monitor.pkg $(BUNDLE) internal/webui/dist build/AppIcon.icns build/AppIcon.iconset build/AppIcon.svg
