# ================= CONFIG =================
APP_NAME := k2-qr
BINARY := app
GO := go

# Đường dẫn FE bạn cung cấp
FE_DIR := /Users/khanglp/Documents/devzone/staging/2k/www

# Timestamp auto
TIMESTAMP := $(shell date +"%Y%m%d_%H%M%S")
MSG ?= update_$(TIMESTAMP)

# ================= GO COMMANDS =================

.PHONY: help start dev build run tidy clean render-build deploy deploy-all dist

help:
	@echo "========= AVAILABLE COMMANDS ========="
	@echo "make start        - Run Go server (local)"
	@echo "make build        - Build Go binary"
	@echo "make run          - Run built binary"
	@echo "make tidy         - go mod tidy"
	@echo "make clean        - Remove binary"
	@echo "make deploy       - Git push (trigger Render deploy)"
	@echo "make deploy-all   - Build FE + Push + Deploy Render"
	@echo "make dist         - Build Frontend"
	@echo "make render-build - Build like Render environment"

# ================= LOCAL DEVELOPMENT =================

start:
	@echo "🚀 Starting Go server (dev)..."
	$(GO) run main.go

dev: start

run:
	@echo "▶️ Running compiled binary..."
	./$(BINARY)

# ================= BUILD =================

build:
	@echo "🔨 Building Go binary..."
	$(GO) build -o $(BINARY) main.go

render-build:
	@echo "🏗 Building (Render compatible)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o $(BINARY) main.go

tidy:
	@echo "📦 Running go mod tidy..."
	$(GO) mod tidy

clean:
	@echo "🧹 Cleaning binary..."
	rm -f $(BINARY)

# ================= FRONTEND =================

dist:
	@echo "📦 Building Frontend (dist-wh)..."
	@$(MAKE) -C $(FE_DIR) dist-wh

dist-app:
	@echo "📦 Building Frontend (dist-app)..."
	@$(MAKE) -C $(FE_DIR) dist-app

# ================= GIT + RENDER DEPLOY =================

git:
	@echo "📌 Staging changes..."
	git add .
	@echo "📝 Commit message: $(MSG)"
	git commit -m "$(MSG)" || echo "⚠️ Nothing to commit"
	@echo "📡 Pushing to GitHub..."
	git push

deploy: git
	@echo "🚀 Code pushed! Render will auto-deploy..."

# Build FE trước rồi deploy (chuẩn production)
deploy-all:
	@echo "📦 Step 1: Build Frontend..."
	@$(MAKE) dist
	@echo "📌 Step 2: Git push..."
	@git add .
	@git commit -m "$(MSG)" || echo "⚠️ Nothing to commit"
	@git push
	@echo "🌍 Step 3: Trigger Render auto deploy..."
	@echo "🎉 DONE! Waiting Render build..."

# ================= EXPORT (giữ lại bản của bạn, tối ưu nhẹ) =================

export:
	@echo "Generating portable create.sh (text-only, UTF-8, skip binary) ..."
	@echo '#!/bin/bash' > ./create.sh
	@echo 'set -e' >> ./create.sh
	@echo 'APP_NAME="apps"' >> ./create.sh
	@echo 'echo "Recreating project: $$APP_NAME..."' >> ./create.sh
	@echo 'rm -rf "$$APP_NAME" && mkdir -p "$$APP_NAME" && cd "$$APP_NAME"' >> ./create.sh
	@echo '' >> ./create.sh
	@bash -c ' \
		find . -type f \
		  ! -path "./node_modules/*" \
		  ! -path "./create.sh" \
		  ! -path "./apps*" \
		  ! -path "./.git/*" \
		  ! -name "*.png" ! -name "*.jpg" ! -name "*.jpeg" ! -name "*.gif" \
		  ! -name "*.webp" ! -name "*.svg" ! -name "*.ico" \
		  ! -name "*.pdf" ! -name "*.zip" ! -name "*.tar" ! -name "*.gz" \
		  ! -name "*.exe" ! -name "*.dll" ! -name "*.so" ! -name "*.dylib" \
		  ! -name "*.bin" ! -name "*.dat" \
		| while IFS= read -r file; do \
		  if file "$$file" 2>/dev/null | grep -qE "text|JSON|XML|UTF-8"; then \
		    rel_path=$$(echo "$$file" | sed "s|^\./||"); \
		    dir=$$(dirname "$$rel_path"); \
		    [ "$$dir" != "." ] && echo "mkdir -p \"$$dir\"" >> ./create.sh; \
		    echo "echo \"Creating $$rel_path...\"" >> ./create.sh; \
		    echo "cat > \"$$rel_path\" <<'"'"'EOF'"'"'" >> ./create.sh; \
		    sed "s/$$/\r/" "$$file" | sed "s/\r$$//" >> ./create.sh; \
		    echo "EOF" >> ./create.sh; \
		    echo "" >> ./create.sh; \
		  else \
		    echo "Skipping binary: $$file" >&2; \
		  fi; \
		done \
	'
	@echo 'chmod -R 755 .' >> ./create.sh
	@echo 'echo "Project recreated successfully!"' >> ./create.sh
	@chmod +x ./create.sh
	@echo "create.sh generated (text-only, UTF-8 safe)!"