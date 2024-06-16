# 🗄NanaFS

![unittest workflow](https://github.com/basenana/nanafs/actions/workflows/unittest.yml/badge.svg)
![pages-build-deployment](https://github.com/basenana/nanafs/actions/workflows/pages/pages-build-deployment/badge.svg)

<p align="right">[ English | <a href="https://github.com/basenana/nanafs/blob/main/README_zh.md">简体中文</a> ]</p>

NanaFS is a **Reference Filing System** inspired by the [GTD (Getting Things Done®)](https://gettingthingsdone.com/wp-content/uploads/2014/10/Filing.pdf) methodology,
designed to collect, store, and process information that does not require immediate action but may be useful in the future.

In personal, academic, and professional life, vast amounts of data and materials are scattered across various information silos,
such as Office documents, emails, RSS feeds, and work notes. While isolated data may seem insignificant,
it can become crucial for future projects. Whether it's a paper in progress or a PowerPoint presentation,
data serves specific purposes.

NanaFS treats files as first-class citizens, aiming to build a system 
that enables quick collection, intelligent categorization, complex querying, and AI enhancement.

## 🚩 Key Features

### 🪄 Cloud-Based Storage

NanaFS utilizes cloud-based storage as main backend storage,
supporting not only object storage but also file-hosting service.
With the cloud-based storage feature, NanaFS can have unlimited storage capacity at an affordable price,
and users can access their data saved in the cloud anywhere and anytime.

The following storage options are already supported or planned to be supported:

- **Object Storage**: AWS S3, AlibabaCloud OSS, Cloudflare R2
- **Cloud Drive**: Google Drive, OneDrive, AliyunDrive, BaiduWangpan
- **Other Storage Protocols**: WebDAV

### 🗃 POSIX Compatibility

NanaFS offers a file system interface that complies with the POSIX standard through FUSE.
This makes it easy to mount NanaFS onto the directory tree and manage NanaFS data using tools such as Finder on MacOS
and Linux.

Additionally, NanaFS has passed the majority of pjdfstest's compatibility tests, ensuring compatibility with Linux/Unix
systems.
This means that existing commands and tools can be used to efficiently process data in NanaFS. For specific needs,
custom scripts or programs can also be written to process files in NanaFS.

### ⚙️ File-Centric Workflow

The actual value of data storage lies in its use. To facilitate this,
NanaFS provides a file-centric workflow engine equipped with rule-based automatic file processing capabilities.

With the workflow engine, tasks such as batch file renaming and creating semantic indexing based on file content become
very simple.
This eases data manipulation, makes data no longer "cold," and helps uncover more inherent value in the data.

### 🔌 Plugin Support

NanaFS supports multiple types of plugins to extend its capabilities. Currently, NanaFS primarily supports three types
of plugins:

- **Source Plugin**: synchronizes data periodically from a source address and integrates it into NanaFS. This includes
  aggregating RSS information and filing emails according to the SMTP protocol.
- **Process Plugin**: provides file processing capabilities and enhances the functionality of workflows by extending
  Process Plugins.

### 🔐 Data Security

Data security is a prerequisite for data storage and usage.
NanaFS provides end-to-end encryption from storage to transmission, ensuring that your cloud data cannot be accessed
even if it is stolen by hackers.
Similarly, cloud service providers cannot access or modify your data, ensuring that your data is not leaked or misused.

## 🚀 Usage

NanaFS's usage guidelines, including its parameters and examples of tools and commands, are documented
in [Instructions For Usage](https://github.com/basenana/nanafs/blob/main/docs/usage.md).

## ✉️ Feedback

If you encounter any problems while using NanaFS, whether it's related to usage, bugs, or you have suggestions for new
features,
please feel free to create an issue.