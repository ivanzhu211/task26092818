# 本机离线构建环境：模块缓存只读，使用 tmp 下的可写缓存与本地文件代理。
export GOCACHE=/private/tmp/gocache
export GOMODCACHE=/private/tmp/gomodcache
export GOPROXY=file:///Users/ivan/go/pkg/mod/cache/download
export GOSUMDB=off
export GOFLAGS=-mod=mod
