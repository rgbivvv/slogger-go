.PHONY: all
all:
	go build -o slogger .
	go build -o new_post ./cmd/new_post
