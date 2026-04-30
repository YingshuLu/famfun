mkdir bin 1>/dev/null 2>&1 && echo created bin directory
go build -o bin/cloud ./cmd/cloud && echo built bin/cloud
go build -o bin/home ./cmd/home && echo built bin/home

# ./bin/home --cloud-addr localhost:4433 --video-dir ./videos --stream-dir ./video_streams --thumb-dir ./thumbnails --tls-insecure
# ./bin/cloud --http-addr :9080 --quic-addr :4433 --dist-dir ./dist --tls-cert certs/server.crt --tls-key certs/server.key   