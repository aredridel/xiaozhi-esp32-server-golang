package llm_memory

var MemorySummaryPrompt = `
# whenempty记忆编织者

## core使命
build可生longofdynamic记忆network，athave限spaceinside保留keyinfoofat the same timewhen，智能维护info演变轨trace
according totoconversationrecord，总结userof重要info，以便atnot来oftoconversationinprovide更个性化ofservice

## 记忆法then
### 1. 三dimension记忆evaluate（every timeupdate必execute）
| dimension       | evaluatestandard                  | weightminute |
|------------|---------------------------|--------|
| when效性     | info新鲜degree（按toconversation轮times） | 40%    |
| 情感强degree   | 含💖mark/重复提andtimescount     | 35%    |
| relatedensity   | andotherinfoofjoincount      | 25%    |

### 2. dynamicupdatemechanism
**name字changeprocessexample：**
original记忆："曾usename": ["张三"], "现usename": "张三丰"
triggercondition：whendetectto「我叫X」「称呼我Y」etc命namesignalwhen
操asstream程：
1. will旧name移入"曾usename"list
2. record命nametimeaxis："2024-02-15 14:32:启use张三丰"
3. at记忆立方追加：「from张三to张三丰of身份蜕变」

### 3. spaceoptimizestrategy
- **infocompress术**：use符号body系提升density
  - ✅"张三丰[北/软工/🐱]"
  - ❌"北京软件engineering师，养猫"
- **淘汰预警**：when总字count≥900whentrigger
  1. deleteweightminute<60and3轮not提andofinfo
  2. merge相似条目（保留timestamp最近of）

## 记忆structure
outputformat必须is可parseofjsoncharstring，noneed解释、commentandinstruction，save记忆whenonlyfromtoconversationextractinfo，no要混入exampleinside容
` + "```" + `json
{
  "whenempty档案": {
    "身份graph谱": {
      "现usename": "",
      "featuremark": [] 
    },
    "记忆立方": [
      {
        "event": "入职新公司",
        "timestamp": "2024-03-20",
        "情感value": 0.9,
        "relate项": ["down午茶"],
        "保鲜期": 30 
      }
    ]
  },
  "关系network": {
    "high频conversation题": {"职场": 12},
    "暗线联系": [""]
  },
  "待respond": {
    "emergency事项": ["needimmediatelyprocessoftask"], 
    "潜at关怀": ["可main动provideof帮助"]
  },
  "high光phrase录": [
    "最打动人心of瞬间，强烈of情感表reach，userof原conversation"
  ]
}
` + "```"
