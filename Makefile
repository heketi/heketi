#
# Based on http://chrismckenzie.io/post/deploying-with-golang/
#

.PHONY: version all run dist clean
	
APP_NAME := heketi
SHA := $(shell git rev-parse --short HEAD)
BRANCH := $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))
VER := $(shell git describe)
ARCH := $(shell go env GOARCH)
DIR=.

ifdef APP_SUFFIX
  VERSION = $(VER)-$(subst /,-,$(APP_SUFFIX))
else 
  VERSION = $(VER)-$(BRANCH)
endif

# Go setup
GO=go
TEST=go test

# Sources and Targets
EXECUTABLES :=$(APP_NAME)
# Build Binaries setting main.version and main.build vars
LDFLAGS :=-ldflags "-X main.HEKETI_VERSION $(VERSION)"
# Package target
PACKAGE :=$(DIR)/dist/$(APP_NAME)-$(VERSION)-$(ARCH).tar.gz

.DEFAULT: all

all: $(EXECUTABLES)

# print the version
version:
	@echo $(VERSION)

# print the name of the app
name:
	@echo $(APP_NAME)

# print the package path
package:
	@echo $(PACKAGE)

$(APP_NAME): 
	$(GO) build $(LDFLAGS) -o $@ $<

run: $(APP_NAME)
	./$(APP_NAME)

test: 
	@$(TEST) -v ./...

clean:
	@echo Cleaning Workspace...
	rm -rf $(APP_NAME)
	rm -rf dist

$(PACKAGE): all
	@echo Packaging Binaries...
	@mkdir -p tmp/$(APP_NAME)
	@cp $(APP_NAME) tmp/$(APP_NAME)/
	@mkdir -p $(DIR)/dist/
	tar -cf $@ -C tmp $(APP_NAME);
	@rm -rf tmp

dist: $(PACKAGE)