package model

// Gateway 是业务层面对模型能力的统一入口。
// 当前 Router 已承担路由、超时、重试和 metadata 归一化职责，先用类型别名保持代码精简。
type Gateway = Router
