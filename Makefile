
.PHONY: build
build: check-go
# $(CGO_OPTS) go build  $(RACE_OPT) $(GOLDFLAGS) -o $(BIN_NAME) ./cmd/mo-service
	GOEXPERIMENT=cgocheck2 go build main.go
	./main

.PHONY: check-go
# 检查Golang是否安装，以及版本是否为1.21以上
check-go:
	@echo "检查Golang环境..."
	@which go > /dev/null 2>&1 || (echo "错误：Golang未安装，请先进行安装。" && exit 1)
	@echo "检测到Golang。检查版本..."
	@$(eval GO_VERSION=$(shell go version | awk '{print $$3}'))
	@$(eval GO_VERSION_NUMBER=$(shell echo $(GO_VERSION) | sed -e 's/go//g'))
	@$(eval MAJOR=$(shell echo $(GO_VERSION_NUMBER) | cut -d. -f1))
	@$(eval MINOR=$(shell echo $(GO_VERSION_NUMBER) | cut -d. -f2))
	@if [ $(MAJOR) -eq 1 ] && [ $(MINOR) -ge 21 ]; then \
		echo "Golang版本为 $(GO_VERSION_NUMBER)，符合要求。"; \
	else \
		echo "错误：Golang版本为 $(GO_VERSION_NUMBER)。请安装1.21以上的版本。" && exit 1; \
	fi


.PHONY: clean
clean: 
	rm main