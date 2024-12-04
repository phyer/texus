
## Init

cd submodule 
git submodule add baidu:/root/repos/go/okexV5Api okex
cd ../
git pull
git submodule init
git submodule update --force --recursive --init --remote
go mod tidy
go mod vendor


## How TO RUN

go run main.go

环境分成两个：test和production，相关配置都在basicConfig.json中。

## 架构


### 欧易文档首页：

https://www.okx.com/docs-v5/zh/#overview
