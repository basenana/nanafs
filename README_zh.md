# NanaFS

![unittest workflow](https://github.com/basenana/nanafs/actions/workflows/unittest.yml/badge.svg)
![pages-build-deployment](https://github.com/basenana/nanafs/actions/workflows/pages/pages-build-deployment/badge.svg)

<p align="right">[ <a href="https://github.com/basenana/nanafs/blob/main/README.md">English</a> | 简体中文 ]</p>

NanaFS 是一款文件系统风格的工作流引擎，也是一款尝试将结构化和非结构化数据的统一管理的数据仓库。

在个人工作、学习和生活中，大量的数据和材料存储在不同的信息孤岛之中，例如 Office 文件、电子邮件、RSS、工作笔记、待办记事等。
孤立的数据没有价值，无论是一个刚起步的项目还是酝酿一篇论文，数据是为具体场景服务的。 我曾花费很多时间编写胶水脚本，使它们可以被一起管理。
为什么不构建一个内建工作流，并提供具体场景数据入口的文件系统呢，于是 NanaFS 应运而生。

NanaFS 采用一个统一的文件模型，将来自多种数据源的数据汇总至同一位置，并基于统一的存储提供通用的工作流能力。

## 核心特性

### 基于云端存储

NanaFS 采用云端存储技术作为其主要后端存储方式，不仅仅支持对象存储，也支持网络硬盘。因此，凭借云端存储的能力，NanaFS
不但可以价格可控的拥有近乎无限的存储容量，也使得用户可以随时随地访问其保存在云端的数据。

已经支持或者计划支持的存储包括：

- **对象存储类**：AWS S3、阿里云OSS、Cloudflare R2
- **云盘类**：Google Drive、OneDrive、阿里云盘、百度网盘
- **其他存储协议**：WebDAV

### POSIX 兼容

NanaFS 通过 FUSE 提供了符合 POSIX 标准的文件系统接口。这使得在 Linux 和 MacOS 操作系统中，可以轻松地将 NanaFS
挂载到目录树上，并通过访达等工具管理 NanaFS 中的数据。

同时，NanaFS 通过了绝大多数的 pjdfstest 兼容性测试，保证了与 Linux/Unix 系统的兼容，而这也意味着可以利用现有的命令和工具对
NanaFS 中的数据进行处理。
对于特定需求，也可以通过编写处理文件的脚本或程序，实现自定义处理 NanaFS 中的文件。

### 面向文件的工作流

数据存储的实际价值在于数据的使用。NanaFS 为此提供了面向文件的工作流引擎，并配备了基于规则的文件自动处理能力。

基于工作流的能力，无论是文件的批量重命名，还是创建基于文件内容语义的索引都变得十分简单。轻松地操控数据，让数据不再冰冷，发掘数据中蕴含的更多价值。

### 支持插件

NanaFS 通过支持多种类型的插件，以实现对功能的拓展。目前 NanaFS 主要支持三种类型的插件：

- **Source Plugin**：定期的从源地址同步数据并收纳到 NanaFS 中，比如聚合 RSS 信息，根据 SMTP 协议归档电子邮件；
- **Mirror Plugin**：将外部存储系统映射到 NanaFS 中，使得 NanaFS 可以统一入口的管理多个存储系统的数据；
- **Process Plugin**：提供文件处理能力，通过拓展 Process Plugin，增强 Workflow 的功能。

### 数据安全

数据安全是进行数据存储和使用的前提条件。NanaFS
提供从存储到传输的全链路加密，即使您的云端数据被窃取，黑客也无法获取您的数据。同样，云服务提供商也无法访问和修改您的数据，保证了您的数据不会被泄露和滥用。

## 使用

NanaFS 的使用方式参数文档 [Instructions For Usage](https://github.com/basenana/nanafs/blob/main/docs/usage_zh.md)。

## 反馈

如果您在使用NanaFS时遇到任何问题，无论是相关于使用方式、Bugs，还是您有新功能的建议，请随时创建 issue。