Processor用户手册
====

### HTTPHeadProcessor - http请求头处理器

#### Key

sh

#### Action

|action| params | 备注 |
| ---- | ----- | ---- |
|`print` | N/A | 打印当前字段值| 
| `set` | `value: string` | 设置该请求头的值 `http.Header.Set()`|
| `add` | `value: string` | 把该值添加到该请求头下`http.Header.Add()` |
| `delete` | N/A | 从请求头删除该键值对|
| `get_from_session` | `key: string` | 从`*Session`查询key的值，并设置值请求头 |
| `store_into_session` | `key: string` | 取出请求头的值并存入`*Session`，键值为`key` |
| `rand_string` | `length: int` | 生成长度为length的随机字符串，设置至请求头 |
|`current_timestamp`| `precision: string` | 生成当前的时间戳，精度为precision, 设置至请求头|
| `uuid` | N/A | 生成uuid，设置至请求头|
| `regexp_picker` | `expression: string; index:int; key: string` | 正则表达式提取请求头内容，index为取值下角标，并存入`*Session`,键值为key |


示例：
```json
[
    {
      "key": "User-Agent",
      "actions": [
        {"action": "set", "params":  ["MicroMessage"]},
        {"action": "print"}
      ]
    }
]
```

#### Assert

尚未施工

### URLProcessor - http URL处理器(不包括query string)

#### Key

只支持以下key

- `host`: http请求的host，包含了端口部分
- `scheme`: http | https
- `path`: 请求路径，不包含query string以及fragment
- `*`: 没有任何作用

#### Action

|action| params | 备注 |
| ---- | ----- | ---- |
| `print` | N/A | 打印当前字段值| 
| `set` | `value: string` | 将Key的值设置为value|
| `get_from_session` | `key: string` | 从`*Session`查询key的值，并设置成当前Key的值 |
| `store_into_session` | `key: string` | 取出Key的值并存入`*Session`，键值为`key` |
| `rand_string` | `length: int` | 生成长度为length的随机字符串，设置值 |
| `current_timestamp`| `precision: string` | 生成当前的时间戳，精度为precision, 设置值|
| `uuid` | N/A | 生成uuid，设置至请求头|
| `reverse_path_params` | `style: string` | 解析解析restful风格的path，变量部分使用`{}`会反转|
| `parse_path_params` | `style: string` | 解析restful风格的path，变量部分使用`{}`，并把解析出来的变量键值对存入`*Session` |
| `set_path_params` | `style: string; values: string ...` | 解析restful风格的path，把style里面所有{}的部分依次替换为values |
| `set_path_params_by_session_keys` | `style: string; key: string ...`| 解析restful风格的path，把style里面所有{}依次替换为在`*Session`查找keys，并设置到path|
| `set_path_params_by_mask` | `style: string; expression: string ...`| 解析restful风格的path，把style里面所有{}依次替换为以`$0`作为原字段的占位符，如原值为123，表达式为hello_$0,则修改为hello_123|

```json
[{
  "key": "host",
  "actions": [
    {"action":  "set", "params":  ["api.github.com"]}
  ]
},
{
  "key": "path",
  "actions": [
    {"action":  "parse_path_params", "params": ["/upay/{org}/actions/{action}"]},
    {"action":  "set_path_params_by_session_keys", "params": ["/newpath/orgs/{org}/actions/{action}", "org", "action"]}
  ]
}
]
```



#### Assert

尚未施工

### JSONProcessor - JSON对象处理器

#### Key

基于<https://github.com/buger/jsonparser>实现，key的支持能力取决于该类库

如`person.name.fullName`或`person[0].name`

#### Action

|action| params | 备注 |
| ---- | ----- | ---- |
| `print` | N/A | 打印当前字段值，如果key为`*`, 打印完整对象| 
| `set` | `value: any` | 将Key的值设置为value, 支持json所支持的任意类型 |
| `delete` | N/A | 删除指定字段 |
| `get_from_session` | `key: string` | 从`*Session`查询key的值，并设置成当前Key的值,请注意:**从Session中取出的对象必须是`[]byte`类型** |
| `store_into_session` | `key: string` | 取出Key的值并存入`*Session`，键值为`key`, **存入的对象必须是`[]byte`类型** |
| `json_context` | `key: string` | 从json对象中取出另外个字段(`key`)的值设置 |
| `rand_string` | `length: int` | 生成长度为length的随机字符串，设置值 |
| `current_timestamp`| `precision: string; type: string` | 生成当前的时间戳，精度为precision, 设置时可以通过`type` （`"string"|"int"`）来控制写入对象类型，默认string|
| `uuid` | N/A | 生成uuid，设置至请求头|
| `reverse` | N/A | 从json对象中取出key对应的值并且反转，并且设置回去|
| `build_string` | `expression: string` | 使用`$0`作为原字段值的占位符，如原值为**123**，表达式为`hello_$0`,则修改为`hello_123` | 



#### Assert


|assert| params | 备注 |
| ---- | ----- | ---- |
| `eq` | `other: any`| 断言是否相等，可支持string、int、float类型 | 
| `ne` | `other: any` | 断言是否不相等，可支持string、int、float类型 |
| `gt` | `other: number` | 断言key的值是否大于other的值，支持int、float类型 |
| `ge` | `other: number` | >= |
| `lt` | `other: number` | < |
| `le` | `other: number` | <= |
| `exist` | N/A | 只判断key是否存在，无论值是什么 |
| `not_exist` | N/A | 判断key是否不存在 |
| `true` | N/A | 判断key的值是否为true |
| `false` | N/A | 判断key的值是否为false |
| `null` | N/A | 判断key的值是否为null |
| `not_null` | N/A | 判断key的值是否非null |
| `empty` | N/A | 判断key的值是否为空字符串 |
| `not_empty` | N/A | 判断key的值是否不为空 |
| `in` | `key: string ...` | 字段值是否在key中，暂支持string类型 |
| `not_in` | `key: string...` | 字段值是否不在key中，暂支持string类型 |

### URLQueryProcessor

#### Key

如以下URL `https://www.baidu.com/s?ie=utf-8&f=8&rsv_bp=0&rsv_idx=1&tn=baidu&wd=havok`，**QueryString**指的是`ie=utf-8&f=8&rsv_bp=0&rsv_idx=1&tn=baidu&wd=havok`部分。

每个键值对以`%s=%s`形式存在，因此示例中的键值对可整理为：

```json
{"ie": "utf-8", "f": "8", "rsv_bp": "0", "rsv_idx": "1", "tn":  "badidu", "wd":  "havok"}
```

所以对于`URLQueryProcessor`的key即为`ie`、`wd`这样的值

#### Action

|action| params | 备注 |
| ---- | ----- | ---- |
| `print` | N/A | 打印当前字段值| 
| `set` | `value: string` | 设置值 `url.Values.Set()`|
| `reverse` | N/A | 将key对应的值反转|
| `add` | `value: string` | 添加值`url.Values.Add()` |
| `delete` | N/A | 删除指定的字段 `url.Values.Delete()`|
| `get_from_session` | `key: string` | 从`*Session`查询key的值，并设置 |
| `store_into_session` | `key: string` | 取出请求头的值并存入`*Session`，键值为`key` |
| `rand_string` | `length: int` | 设置值为指定长度的随机字符串 |
| `current_timestamp`| `precision: string` | 生成当前的时间戳，精度为precision, 并设置 |
| `uuid` | N/A | 设置值为uuid|

#### Assert

施工中

### FormBodyProcessor

#### Key

所有的form提交的body均会转换成`url.Values`对象，并处理(代码上逻辑上和URLQueryProcessor处理类似)

#### Action

|action| params | 备注 |
| ---- | ----- | ---- |
| `print` | N/A | 打印当前字段值| 
| `set` | `value: string` | 设置值 `url.Values.Set()`|
| `reverse` | N/A | 将key对应的值反转|
| `add` | `value: string` | 添加值`url.Values.Add()` |
| `delete` | N/A | 删除指定的字段 `url.Values.Delete()`|
| `get_from_session` | `key: string` | 从`*Session`查询key的值，并设置 |
| `store_into_session` | `key: string` | 取出请求头的值并存入`*Session`，键值为`key` |
| `rand_string` | `length: int` | 设置值为指定长度的随机字符串 |
| `current_timestamp`| `precision: string` | 生成当前的时间戳，精度为precision, 并设置 |
| `uuid` | N/A | 设置值为uuid|

#### Assert

施工中

### HTMLProcessor

#### Key

目前针对HTMLProcessor所有操作Key都是没用的，可以设置为`.`, 大部分操作都是围绕正则进行

#### Action

|action| params | 备注 |
| ---- | ----- | ---- |
| `regexp_picker` | `expression: string; index:int; key: string` | 正则表达式提取请求头内容，index为取值下角标，并存入`*Session`,键值为key |

#### Assert

|assert| params | 备注 |
| ---- | ----- | ---- |
| `contain` | values: string... | 所有values的值都必须被包含|

### ExtraProcessor

#### Assert

尚未施工(目前暂未考虑该功能实现)