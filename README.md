# QMediaSync

![GitHub release (latest by date)](https://img.shields.io/github/v/release/qicfan/qmediasync)

## 讨论方式

- 电报群：[http://t.me/q115_strm](https://t.me/q115_strm)
- QQ群：1057459156
- Meow官方频道：使用鸿蒙系统手机扫描下方二维码来关注频道（请用官方浏览器打开）
  
  <img src="https://s.mqfamily.top/meow.png" width="200" />

### 开源版本不包含115开放平台账号，需要自备

### 本项目接受除了资源（搜索、订阅、下载）、逆向接口的一切功能PR

#### PR以后如果没有动静可以邮件、TG、QQ联系作者

## 介绍

- **默认用户名 admin,密码 admin123**
- 默认端口：http-12333   https-12332
- emby代理端口默认：http-8095  https-8094
- 其他见 [wiki](https://github.com/qicfan/qmediasync/wiki)

## 调试启动

```bash
go run .
```

## 退出

- linux: ```ctrl + c```
- windows: 系统托盘找到QMediaSync图标，右键退出

## 编译且发布新版本

```bash
cd build_scripts
sudo ./build_and_release.sh -v vx.xx.xx
```

编译要求具有github命令行gh权限，且已经登录
如果要发布docker镜像，需要提前登录docker hub
该命令会编译打包所有平台的二进制文件，生成release版本，并且发布到github release页面，推送到docker hub

## 数据库

开源版本不包含postgres数据库二进制文件，需要自己安装，建议版本15.x，然后配置环境变量使用。详见wiki中的[安装](https://github.com/qicfan/qmediasync/wiki/Linux-%E5%AE%89%E8%A3%85%E4%BD%BF%E7%94%A8)

## 需要自备的密钥

- 115开放平台 AppID，可在web页面设置
- TMDB API KEY，可在web页面设置
- OpenAI兼容的 API KEY，目前用的硅基流动，可在web页面设置
- Fanart.tv API KEY

项目中搜索// TODO 开源版本留空 可以找到相关位置

## 配套前端

- [QMediaSync-Frontend](https://github.com/qicfan/q115-strm-frontend)

## 贡献者

![Contributors](https://contrib.rocks/image?repo=qicfan/qmediasync)

## Star

![Star History](https://api.star-history.com/svg?repos=qicfan/qmediasync&type=Date)

## 请作者喝杯咖啡

![请作者喝杯咖啡](http://s.mqfamily.top/alipay_wechat.jpg)
