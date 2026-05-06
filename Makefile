.PHONY: all proto frontend cloud home clean run-cloud run-home dev-cert test

all: proto frontend cloud home

proto:
	protoc --go_out=. --go_opt=paths=source_relative proto/famfun.proto
	mv proto/famfun.pb.go pkg/proto/famfun.pb.go

cloud:
	go build -o bin/cloud ./cmd/cloud

home:
	go build -o bin/home ./cmd/home

test:
	go test ./...

clean:
	rm -rf bin/ dist/
	cd frontend && rm -rf node_modules dist

run-cloud: cloud
	./bin/cloud --http-addr :8080 --quic-addr :4433 --dist-dir ./dist

run-home: home
	./bin/home --cloud-addr localhost:4433 --video-dir ./videos --tls-insecure

dev-cert:
	mkdir -p certs
	openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
		-keyout certs/server.key -out certs/server.crt -days 365 -nodes \
		-subj "/CN=localhost"
