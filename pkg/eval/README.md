1. [mockjs examples](http://mockjs.com/examples.html)
1. [MockJS快速入门](https://juejin.cn/post/6844903860343963655)
1. [Mockjs,再也不用追着后端小伙伴要接口了](https://juejin.cn/post/6844903492235034632)


|                    |         String          |                  Number                   |                     Boolean                      | Undefined |  Null   |                                    Object                                     |                      Array                       |    Function    |
|--------------------|-------------------------|-------------------------------------------|--------------------------------------------------|-----------|---------|-------------------------------------------------------------------------------|--------------------------------------------------|----------------|
|      `|min-max`      | 字符串重复min-max次后拼接得出新的字符串 |               随机得到min-max的值               | min/(min+max)概率生成value值，max/(min+max)概率生成!value值 | 当前数据类型无效  | 返回null值 | 先在min-max中随机生成一个数值value，然后选取该对象的value个属性出来组成一个新的对象，若value大于该对象的属性个数，则将所有属性拿出来 | 先在min-max中随机生成一个数值value，然后将数组元素重复value次然后合并为一个数组 | 直接执行函数并返回了函数的值 |
|       `|count`       |   字符串重复count次得出新的字符串    |              生成一个值为count的数值               |   (count-1)/count概率生成value值，1/count概率生成!value值   | 当前数据类型无效  | 返回null值 |              选取该对象的count个属性出来组成一个新的对象，若count大于该对象的属性个数，则将所有属性拿出来              |              将数组元素重复count次然后合并为一个数组              | 直接执行函数并返回了函数的值 |
| `|min-max.dmin-dmax` |      与规则|min-max相同      | 生成一个浮点数，浮点数的整数部分是min-max，小数的位数是dmin-dmax  |                  与规则|min-max相同                   | 当前数据类型无效  | 返回null值 |                                 与规则|min-max相同                                 |                  与规则|min-max相同                   | 直接执行函数并返回了函数的值 |
|  `|min-max.dcount`   |      与规则|min-max相同      |   生成一个浮点数，浮点数的整数部分是min-max，小数的位数是dcount   |                  与规则|min-max相同                   | 当前数据类型无效  | 返回null值 |                                 与规则|min-max相同                                 |                  与规则|min-max相同                   | 直接执行函数并返回了函数的值 |
|  `|count.dmin-dmax`  |       与|count规则相同       | 生成一个浮点数，浮点数的整数部分的值是count，小数的位数是dmin-dmax位 |                   与|count规则相同                    | 当前数据类型无效  | 返回null值 |                                  与|count规则相同                                  |                   与|count规则相同                    | 直接执行函数并返回了函数的值 |
|   `|count.dcount`    |       与|count规则相同       |  生成一个浮点数，浮点数的整数部分的值是count，小数的位数是dcount位   |                   与|count规则相同                    | 当前数据类型无效  | 返回null值 |                                  与|count规则相同                                  |                   与|count规则相同                    | 直接执行函数并返回了函数的值 |
|       `|+step`       |     无作用，将value直接返回      |  初始值为预设的value值，每重新请求一次时数值value会增加一个step值  |                  无作用，将value值返回                   | 当前数据类型无效  | 返回null值 |                                 无作用，将value值返回                                 |   初始值为下标是预设的value的值，每重新请求一次时，下标value会增加一个step值   | 直接执行函数并返回了函数的值 |

```js
// 使用 Mock
let Mock = require('mockjs');
Mock.mock('http://1.json','get',{
    // 属性 list 的值是一个数组，其中含有 1 到 3 个元素
    'list|1-3': [{
        // 属性 sid 是一个自增数，起始值为 1，每次增 1
        'sid|+1': 1,
        // 属性 userId 是一个5位的随机码
        'userId|5': '',
        // 属性 sex 是一个bool值
        "sex|1-2": true,
        // 属性 city对象 是对象值中2-4个的值
        "city|2-4": {
            "110000": "北京市",
            "120000": "天津市",
            "130000": "河北省",
            "140000": "山西省"
        },
        //属性 grade 是数组当中的一个值
        "grade|1": [
            "1年级",
            "2年级",
            "3年级"
        ],
        //属性 guid 是唯一机器码
        'guid': '@guid',
        //属性 id 是随机id
        'id': '@id',
        //属性 title 是一个随机长度的标题
        'title': '@title()',
        //属性 paragraph 是一个随机长度的段落
        'paragraph': '@cparagraph',
        //属性 image 是一个随机图片 参数分别为size, background, text
        'image': "@image('200x100', '#4A7BF7', 'Hello')",
        //属性 address 是一个随机地址
        'address': '@county(true)',
        //属性 date 是一个yyyy-MM-dd 的随机日期
        'date': '@date("yyyy-MM-dd")',
        //属性 time 是一个 size, background, text 的随机时间
        'time': '@time("HH:mm:ss")',
        //属性 url 是一个随机的url
        'url': '@url',
        //属性 email 是一个随机email
        'email': '@email',
        //属性 ip 是一个随机ip
        'ip': '@ip',
        //属性 regexp 是一个正则表达式匹配到的值 如aA1
        'regexp': /[a-z][A-Z][0-9]/,
    }]
})
```
