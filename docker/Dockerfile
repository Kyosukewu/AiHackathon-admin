# 使用官方的 Go 基礎映像
FROM golang:1.24-alpine

RUN apk update \
    && apk upgrade --no-cache

# 設置工作目錄
WORKDIR /app

# 將 go.mod 和 go.sum 複製到容器中
COPY ./go.mod ./go.sum ./

# 下載依賴
RUN go mod tidy

# 複製專案檔案
COPY ./ .

# 編譯 Go 應用程式
RUN go build -o AiHackathon-admin ./cmd/app/main.go

# 執行 Go 應用程式
CMD [ "/app/AiHackathon-admin", "--migrate" ]