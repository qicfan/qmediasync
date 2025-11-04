## 讨论方式
- 电报群：[http://t.me/q115_strm](https://t.me/q115_strm)
- QQ群：1057459156

#### 从v.10.x版本升级到 v0.11.x或以上版本时，如果无法启动，则删除docker容器，删除本地镜像，然后重新构建compose.

## 介绍
- **默认用户名 admin,密码 admin123**
- 默认端口：http-12333   https-12332
- emby代理端口默认：http-8095  https-8094
- 支持的同步源：
  - CD2本地挂载
  - NAS系统的远程挂载
  - OpenList
  - 115开放平台
- 核心功能：
  - STRM生成
  - 元数据下载
  - 元数据上传
  - 播放链接解析
  - emby外网302
  - 多台设备共用115网盘生成的strm和元数据（需要自己实现数据同步，具体参见最下方的教程）
  - SSL支持（如果你在公网主机部署emby和qmediasync，强烈建议开启ssl）
  - emby媒体信息提取（部分代替神医助手）
  - 刮削 & 整理
 
## 特点：
- 免费
- 支持添加多个115或者openlist账号
- 如果进行了本地刮削，在同步时会自动将元数据上传到网盘
- 内置emby的外网302（直链）播放，使用8095端口代理emby
- 内置115下载链接代理，用于解决部分客户端因为UA问题导致不能播放的问题
- 智能判断是否启用115下载链接代理，8095端口的emby会直接重定向到115下载链接，8096端口的emby会通过代理访问115下载链接，基本解决UA问题导致的无法播放问题
-  115使用开放平台接口，不担心风控，不担心暴毙，且做了对应的性能优化，3W多文件的库全量大概10分钟，增量30秒左右可以完成同步
- 同步时按照目录划分，可以添加多个不同或者同类型的同步目录
- 使用定时任务进行同步，最小间隔半小时，如果内容变更不频繁可以关闭某个同步目录的定时任务
- 每次同步时会检查元数据的大小，如果网盘和本地文件大小不同，则会下载网盘文件到本地
- 支持电报通知
- 支持windows，linux，macos可执行文件直接运行
- 支持docker镜像运行
- 支持amd64和arm64架构
- https支持
- 支持添加多种来源的目录来刮削和整理
- 基于postgres数据库构建，解决sqlite可能死锁和性能低下问题，postgres完全内建开箱即用，也可以连接已有的外部数据库

## 缺点：
- 由于使用开放平台接口，所以每次同步其实都全量查询了115文件列表，所以速度天生不可能快
- 由于速度不可能快，所以增加了缓存，所有增量同步都是基于缓存的；所以导致的结果是无法感知到115的文件夹重命名或者移动，当然不影响文件的新增和改动
- openlist和本地挂载模式没有办法优化所以同步效率不行，但是可以借此支持更多网盘。
- 不开源（开源的话很容易被人拿走改改收费）

### 功能列表

见project需求列表：[https://github.com/users/qicfan/projects/3/views/1](https://github.com/users/qicfan/projects/3/views/1)

## 快速开始

### 使用 Docker
[Docker安装](https://github.com/qicfan/qmediasync/wiki/Docker%E5%AE%89%E8%A3%85)

### 不使用 Docker
[非Docker安装](https://github.com/qicfan/qmediasync/wiki/%E9%9D%9EDocker%E5%AE%89%E8%A3%85)

## 首次使用

1. 启动容器后访问: http://your-ip:12333
2. 默认登录用户：admin，默认密码：admin123
3. 如果不是很了解，所有配置全部保持默认值
4. 如果要使用网盘：系统设置-网盘账号管理-添加账号，添加完后在下放的卡片中点击授权按钮进行授权
5. 在同步-同步目录，点击添加同步目录
6. 添加完成后，下放卡片列表会显示新添加的同步目录
7. 如果该目录内的资源变动概率较小，建议关闭定时同步，在变动时手动点击 启动同步

具体可以看该帖子：[https://club.fnnas.com/forum.php?mod=viewthread&tid=38393&extra=page%3D1](https://club.fnnas.com/forum.php?mod=viewthread&tid=38393&extra=page%3D1)

## 数据

重要数据位于 `/app/config` 目录，该目录的具体位置由个人映射决定，请定期备份

## SSL支持
- 将server.crt和server.key放入/app/config目录（这是容器内目录，要放到对应的宿主机目录内）
- server.crt要包含完整证书链，如果用acme.sh，则参考如下命令
```bash
acme.sh --install-cert -d your_domain \
--cert-file      /mnt/docker/qmediasync/config/server.cert  \
--key-file       /mnt/docker/qmediasync/config/server.key  \
--fullchain-file /mnt/docker/qmediasync/config/server.crt \
```
- 上面的/mnt/docker/qmediasync/config替换为你docker配置中的目录
- 然后https监听在12332端口，http服务监听在12333端口
- STRM设置-STRM直连地址 建议依然使用12333的http服务，因为不对外所以兼容性更高
- **目前证书变更后不会热更新，请手动重启容器或服务**

## 115网盘多设备共用一套数据
- 目前内置两个115开放平台App ID，一个App Id可以授权两个设备，总共可以支持4台设备
- 如果依然不够，可以自己申请App Id，本项目支持自定义App Id，路径：网盘账号管理 - 添加账号 - 网盘类型：115网盘 - 开放平台应用：自定义 - App Id: 输入自己申请的App ID
- 多设备可以共用一套Strm和元数据
  - strm设置 - strm直连地址: 这里输入 **http://your_domain:12333**，可以是假域名，使用方自己做hosts
  - 指定一台设备为主设备，主设备生成strm，然后使用同步工具（比如：微力同步）将生成的strm和元数据同步到其他设备
  - 其他设备添加网盘账号即可，注意：网盘账号ID必须和主设备的网盘账号ID相同，比如都是1
  - 每台设备创建docker容器时，使用如下compose，主要是增加了your_domain的hosts，如果你有自己的域名且可以解析就忽略下面
```
services:
    qmediasync:
        image: qicfan/qmediasync:latest
        container_name: qmediasync
        restart: unless-stopped
        extra_hosts:
          - "your_domain:127.0.0.1"
        ports:
            - "12333:12333"
            - "12332:12332"
            - "8095:8095"
            - "8094:8094"
        volumes:
            - /vol1/1000/docker/qmediasync/config:/app/config
            - /vol2/1000/网盘:/media
        environment:
            - TZ=Asia/Shanghai
            - DB_HOST=localhost
            - DB_PORT=5432
            - DB_USER=qms
            - DB_PASSWORD=qms123456
            - DB_NAME=qms
```
  - 如果使用emby，且未使用外网302，需要让emby也能访问到your_domain
    - 如果是宿主机直接安装，需要给/etc/hosts或者windows的hosts文件中加入一行：127.0.0.1 your_domain
    - 如果使用容器部署，需要给compose中增加下面的设置，或者在容器设置中增加
```
extra_hosts:
  - "your_domain:qmediasync_ip或宿主机ip"
```

## FAQ

- 如果有服务无法启动或者运行逻辑始终不对，建议删除/app/config/postgres，然后重启容器，注意：该操作会清除所有数据
- /app/config/logs下的内容可以删除，不影响运行
- /app/config/libs下的内容如果删除，不影响运行
- /ap/config/posgres是数据库目录，建议定期备份
- emby外网302目前使用8095和8094端口，暂时不能变动
- 如果使用CD2的本地挂载目录做为同步源，请把完整路径映射进来，比如挂载目录为/vol1/1000/CloudNAS，那么映射路径是：
  - /vol1/1000/CloudNAS:/vol1/1000/CloudNAS
  - emby或者其他媒体服务器中也需要这么映射，必须完整路径一模一样，否则视频无法播放
  - 只有网盘同步源可以使用emby外网302播放，本地目录同步源不可以
- 上传和下载是异步的不会跟着同步任务一起完成，同步任务完成后才会触发上传下载，这时元数据文件可能还没有下载，不要着急关闭容器和软件，请等待
- 建议115qps设置为：下载qps-5，接口qps-3    经过测试，这是最稳定的配置。


## 请作者喝杯咖啡
![请作者喝杯咖啡](http://s.mqfamily.top/alipay_wechat.jpg)



















