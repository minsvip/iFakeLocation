APP_NAME := ifakelocation
BIN_DIR := bin
CMD_DIR := ./cmd/ifakelocation

.PHONY: all build run clean vet test cross-compile help

all: build

## build: 编译当前平台的二进制可执行文件
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "Build complete: $(BIN_DIR)/$(APP_NAME)"

## run: 编译并直接运行应用
run: build
	./$(BIN_DIR)/$(APP_NAME)

## clean: 清理构建目录
clean:
	rm -rf $(BIN_DIR)
	@echo "Cleaned $(BIN_DIR)"

## vet: 代码静态分析检查
vet:
	go vet ./...

## test: 运行单元测试
test:
	go test -v ./...

## cross-compile: 交叉编译多平台发布包 (macOS arm64/amd64, Linux amd64, Windows amd64)
cross-compile: clean
	@mkdir -p $(BIN_DIR)
	@echo "Building for macOS (arm64 - Apple Silicon)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BIN_DIR)/$(APP_NAME)-darwin-arm64 $(CMD_DIR)
	@echo "Building for macOS (amd64 - Intel)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN_DIR)/$(APP_NAME)-darwin-amd64 $(CMD_DIR)
	@echo "Building for Linux (amd64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN_DIR)/$(APP_NAME)-linux-amd64 $(CMD_DIR)
	@echo "Building for Windows (amd64)..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN_DIR)/$(APP_NAME)-windows-amd64.exe $(CMD_DIR)
	@echo "Cross compilation complete! Artifacts are in $(BIN_DIR)/"

## help: 显示所有命令帮助
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':'
