all: 
	go build -ldflags "-linkmode external -extldflags -static"
