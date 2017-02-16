GOVERSION=$(shell go version)
GOOS=$(word 1,$(subst /, ,$(word $(words $(GOVERSION)), $(GOVERSION))))
GOARCH=$(word 2,$(subst /, ,$(word $(words $(GOVERSION)), $(GOVERSION))))
VERSION=$(patsubst "%",%,$(lastword $(shell grep 'const Version' sharaq.go)))
ARTIFACTS_DIR=$(CURDIR)/artifacts/$(VERSION)
RELEASE_DIR=$(CURDIR)/release/$(VERSION)

.PHONY: clean installdeps test $(GAE)/goapp appengine_test

ifneq ($(GAE),)
export PATH := $(GAE):$(PATH)
endif

installdeps: glide-$(GOOS)-$(GOARCH)/glide
	PATH=glide-$(GOOS)-$(GOARCH):$(PATH) glide install

glide-$(GOOS)-$(GOARCH):
	@echo "Creating $(@F)"
	@mkdir -p $(@F)

glide-$(GOOS)-$(GOARCH)/glide:
	@$(MAKE) glide-$(GOOS)-$(GOARCH)
	@wget -O - https://github.com/Masterminds/glide/releases/download/v0.12.3/glide-v0.12.3-$(GOOS)-$(GOARCH).tar.gz | tar xvz
	@mv $(GOOS)-$(GOARCH)/glide glide-$(GOOS)-$(GOARCH)
	@rm -rf $(GOOS)-$(GOARCH)

test:
ifeq ($(GAE),)
	go test -v $(shell glide-$(GOOS)-$(GOARCH)/glide novendor)
else
	$(MAKE) appengine_test
endif

$(GAE)/goapp:
	@mkdir -p .download
	@mkdir -p $(GAE)
	wget -q https://storage.googleapis.com/appengine-sdks/featured/go_appengine_sdk_$(GOOS)_$(GOARCH)-1.9.48.zip
	@mv go_appengine_sdk_$(GOOS)_$(GOARCH)-1.9.48.zip .download/
	@unzip .download/go_appengine_sdk_$(GOOS)_$(GOARCH)-1.9.48.zip > /dev/null
	@mv go_appengine/* $(GAE)

appengine_test: $(GAE)/goapp
	goapp test -v $(shell glide-$(GOOS)-$(GOARCH)/glide novendor)

$(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH):
	@mkdir -p $@

build: $(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH)/sharaq$(SUFFIX)

$(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH)/sharaq$(SUFFIX): $(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH) $(SRC_FILES)
	@echo " * Building binary for $(GOOS)/$(GOARCH)..."
	@go build -o $(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH)/sharaq$(SUFFIX) cmd/sharaq/sharaq.go

all: build-linux-amd64 build-linux-386 build-darwin-amd64 build-darwin-386 build-windows-amd64 build-windows-386

build-windows-amd64:
	@$(MAKE) build GOOS=windows GOARCH=amd64 SUFFIX=.exe

build-windows-386:
	@$(MAKE) build GOOS=windows GOARCH=386 SUFFIX=.exe

build-linux-amd64:
	@$(MAKE) build GOOS=linux GOARCH=amd64

build-linux-386:
	@$(MAKE) build GOOS=linux GOARCH=386

build-darwin-amd64:
	@$(MAKE) build GOOS=darwin GOARCH=amd64

build-darwin-386:
	@$(MAKE) build GOOS=darwin GOARCH=386

$(RELEASE_DIR)/sharaq_$(GOOS)_$(GOARCH):
	@mkdir -p $@

$(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH)/Changes: $(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH) Changes
	@echo " * Copying Changes for $(GOOS)/$(GOARCH)"
	@cp Changes $(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH)

$(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH)/README.md: $(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH) README.md
	@echo " * Copying README.md for $(GOOS)/$(GOARCH)"
	@cp README.md $(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH)

release-changes: $(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH)/Changes
release-readme: $(ARTIFACTS_DIR)/sharaq_$(GOOS)_$(GOARCH)/README.md

release-windows-amd64:
	@$(MAKE) build release-changes release-readme release-zip GOOS=windows GOARCH=amd64

release-windows-386:
	@$(MAKE) build release-changes release-readme release-zip GOOS=windows GOARCH=386

release-linux-amd64:
	@$(MAKE) build release-changes release-readme release-targz GOOS=linux GOARCH=amd64

release-linux-386:
	@$(MAKE) build release-changes release-readme release-targz GOOS=linux GOARCH=386

release-darwin-amd64:
	@$(MAKE) build release-changes release-readme release-zip GOOS=darwin GOARCH=amd64

release-darwin-386:
	@$(MAKE) build release-changes release-readme release-zip GOOS=darwin GOARCH=386

release-tarbz: $(RELEASE_DIR)/sharaq_$(GOOS)_$(GOARCH)
	@echo " * Creating tar.bz for $(GOOS)/$(GOARCH)"
	@tar -cjf $(RELEASE_DIR)/sharaq_$(GOOS)_$(GOARCH).tar.bz2 -C $(ARTIFACTS_DIR) sharaq_$(GOOS)_$(GOARCH)

release-targz: $(RELEASE_DIR)/sharaq_$(GOOS)_$(GOARCH)
	@echo " * Creating tar.gz for $(GOOS)/$(GOARCH)"
	@tar -czf $(RELEASE_DIR)/sharaq_$(GOOS)_$(GOARCH).tar.gz -C $(ARTIFACTS_DIR) sharaq_$(GOOS)_$(GOARCH)

release-zip: $(RELEASE_DIR)/sharaq_$(GOOS)_$(GOARCH)
	@echo " * Creating zip for $(GOOS)/$(GOARCH)"
	@cd $(ARTIFACTS_DIR) && zip -9 $(RELEASE_DIR)/sharaq_$(GOOS)_$(GOARCH).zip sharaq_$(GOOS)_$(GOARCH)/*

release-files: release-windows-386 release-windows-amd64 release-linux-386 release-linux-amd64 release-darwin-386 release-darwin-amd64

release-github-token: github_token
	@echo "file `github_token` is required"

release-upload: release-files release-github-token
	ghr -u $(GITHUB_USERNAME) -t $(shell cat github_token) --draft --replace $(VERSION) $(RELEASE_DIR)

clean:
	@echo "Removing release/$(VERSION) and artifacts/$(VERSION)"
	@rm -rf $(RELEASE_DIR)
	@rm -rf $(ARTIFACTS_DIR)
