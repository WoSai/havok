# Havok - 分布式日志回放系统

[toc]



## 1. 系统架构

**整体系统架构图**

![](https://unmurphy-pic.oss-cn-beijing.aliyuncs.com/20181114163842.png)

**Dispatcher设计**

![](https://my-storage.oss-cn-shanghai.aliyuncs.com/picgo/20190621094608.png)

## 2. 对象说明

### 2.1 Dispatcher

`Dispatcher`在整个架构里承担了日志数据的索引（下载）、排序（有规则约束）、时间控制、请求分发的任务，其主要有四大组件：

#### 2.1.1 Fetcher

`Fetcher`负责对日志数据的索引以及反序列化（`LogRecord`），其定义为

```go
type Fetcher interface{
	TimeRange(begin, end time.Time)
    WithAnalyzer(Analyzer)
    SetOutput(chan<- *LogRecordWrapper)
    SubTask
}
````

其具体实现有以下几种：

- `FileFetcher`: 本地单日志文件采集
- `AliyunSLSConcurrencyFetcher`: 阿里云SLS日志采集，因SLS日志不是严格排序的，该`Fetcher`会重排序一秒以内的日志
- `KafkaSinglePartitionFetcher`： 单Partition的Kafka采集器，无须重排序

未来计划实现以下`Fetcher`:

- `ElasticFetcher`: ElasticSearch的日志收集对象
- `KafkaFetcher`: 完善的Kafka采集器，支持单topic、多partition

##### 2.1.1.1 Analyzer

日志分析器，是Fetcher的组件之一，用于将采集到的单行日志`[]byte`反序列化成`LogRecordWrapper`对象

该interface定义如下：

```go
type Analyzer interface {
    Use(...AnalyzeFunc)
    Analyze([]byte) *LogRecordWrapper
}
```

一般情况下，你只需要实现一个`AnalyzeFunc`即可： 

```go
type AnalyzeFunc func([]byte) (*LogRecordWrapper, bool)
```


#### 2.1.2 TimeWheel

时间轮，负责接收来自`Fetcher`的`LogRecordWrapper`数据并严格按照时间顺序发送给`Havok`

`TimeWheel`时钟递增幅度由`defaultTimeWheelInterval`来控制，当前设置为十毫秒，理论上日志发送会比较顺滑

同时`TimeWheel`也支持快放、慢放（可联想成播放器），由任务的`JobConfiguration.Speed`字段控制

#### 2.1.3 Havok

项目的同名组件，实际为一个gRPC Server，负责与不同的`Replayer`通讯

该服务仅暴露两个rpc接口：

```
service Havok {
    rpc Subscribe (ReplayerRegistration) returns (stream DispatcherEvent) {
    }
    rpc Report (StatsReport) returns (ReportReturn) {  // 返回值标示在哪次request_id中断
    }
}
```

- `Subscribe`: replayer主动调用，dispatcher下发的信令都走DispatcherEvent流
- `Report`: replayer完成压测数据统计后调用

#### 2.1.3.1 ReplayerProxy

该对象赋予了`Havok`定向投递`LogRecord`的能力:

```go
// WithHashFunc 更新hash算法
func (ip *ReplayerProxy) WithHashFunc(ha HashFunc) *ReplayerProxy

type HashFunc func(string) uint32
```

当`LogRecordWrapper.HashField`字段被赋值后（在Analyzer控制），ReplayerProxy对该字段就会进行hash计算，根据计算结果投递到具体的replayer

默认的的`HashFunc`为`RoundTrip`，轮询分发，也内置了`FNV Hash`算法


#### 2.1.4 Report

该组件提供了聚合压测报告以及上报第三方的能力，主要模块是从`ultron`中移植过来：

可以实现以下func来扩展对聚合后报表的处理能力：

```go
type ReportHandleFunc func(types.Report)
```

结合grafana等，能够实现如下的展示效果：

![grafana](https://unmurphy-pic.oss-cn-beijing.aliyuncs.com/20180929182922.png)

## 3. 配置说明

### 3.1 Dispatcher

#### 3.1.1 任务配置

控制回放的时间范围、比例、速率，示例如下：

```toml
[job]
rate = 1.0   # 回放倍数，如2.0，表示一次请求回放两次
speed = 1.0  # 回放速率，如2.0，表示原来10秒内有5次请求发生，回放时会把这些请求在5秒回放完
begin = 1532058494000
end = 1532076494000
```

#### 3.1.2 Fetcher配置

使用`FileFetcher`

```toml
[fetcher]
type = "file"

[fetcher.file]
path = "upay-2018-07-10.0.log"
```

使用sls fetcher:

```toml
[fetcher]
type = "sls"

[fetcher.sls]
access_key_id = "xxxxx"
access_key_secret = "yyyyyyy"
region = "xxxx.log.aliyuncs.com"
project = "havok-project"
logstore = "havok-project"
expression = "arguments=* and message=\"invoking method start\""   # sls查询表达式
concurrency = 8
pre-download = 5000
```

使用kafka fetcher

```toml
[fetcher]
type = "kafka-single-partition"

[fetcher.kafka]
brokers = ["127.0.0.1:9092", "127.0.0.1:9192"]
topic = "havok_project_pressure"
offset = -2    # oldest
```

#### 3.1.3 Service监听配置

dispatcher同时启动了gRPC服务以及http服务，以下配置控制监听的端口

```toml
[service]  # 暂时无效
http = ":16200"
grpc = ":16300"
```

#### 3.1.4 Reporter配置

```toml
[reporter]
[reporter.influxdb]
url = "http://127.0.0.1:8086"
database = "stress-test"
user = ""
password = ""
```


## 4. 运行说明

## 5. 接入说明

