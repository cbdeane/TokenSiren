SHELL := /bin/sh

BPF_DIR := bpf
GEN_DIR := gen
BIN_DIR := bin

BPF_CLANG ?= clang
BPF_CFLAGS ?= -O2 -g -target bpf -D__TARGET_ARCH_x86
BPF_INCLUDE ?= /usr/include

BPF_SRC := $(BPF_DIR)/tracer.c
BPF_OBJ := $(GEN_DIR)/tracer.o

GO ?= go
GO_BIN := $(BIN_DIR)/tokensiren

.PHONY: all build bpf go clean

all: build

build: bpf go

$(GEN_DIR):
	@mkdir -p $(GEN_DIR)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

bpf: $(GEN_DIR) $(BPF_OBJ)

$(BPF_OBJ): $(BPF_SRC) $(BPF_DIR)/common.h
	$(BPF_CLANG) $(BPF_CFLAGS) -I$(BPF_INCLUDE) -I$(BPF_DIR) -c $(BPF_SRC) -o $(BPF_OBJ)

go: $(BIN_DIR)
	$(GO) build -o $(GO_BIN) ./cmd/tokensiren

clean:
	@rm -rf $(GEN_DIR) $(BIN_DIR)
