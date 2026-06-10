export type MatchStatus = "strong" | "partial" | "missing";

export type EvidenceSource = {
  id: string;
  document: string;
  location: string;
  snippets: Array<{
    location: string;
    label: string;
  }>;
  excerpt: string[];
  explanation: string;
};

export type Requirement = {
  id: string;
  group: string;
  text: string;
  status: MatchStatus;
  evidenceCount: number;
  sources: EvidenceSource[];
};

export type RequirementGroup = {
  id: string;
  label: string;
  summaryStatus: MatchStatus;
  summaryEvidenceCount: number;
  requirements: Requirement[];
};

const backendSources: EvidenceSource[] = [
  {
    id: "source-backend",
    document: "01-工作经历/字节跳动-后端开发工程师.md",
    location: "L42-48",
    snippets: [
      { location: "L42-48", label: "Go 并发与服务优化实践" },
      { location: "L88-95", label: "基础组件开发与性能提升" },
      { location: "L102-108", label: "内存优化与稳定性建设" },
    ],
    excerpt: [
      "#### Go 并发与服务优化实践",
      "",
      "在订单服务中，我负责核心链路的开发与性能优化。",
      "基于 Go 的 goroutine 和 channel 实现高并发的",
      "异步处理框架，将下单 QPS 从 2k 提升到 12k+，",
      "同时通过 worker 池、context 超时控制和内存复用，",
      "将 P99 延迟降低了 35%，显著提升了系统稳定性。",
    ],
    explanation:
      "候选人展示了使用 Go 并发模型构建高并发服务的经验，并通过工程手段达成显著性能与稳定性提升，直接满足该要求。",
  },
  {
    id: "source-project",
    document: "02-项目经验/电商平台-交易服务.md",
    location: "L18-29",
    snippets: [
      { location: "L18-29", label: "交易链路拆分与异步化" },
      { location: "L40-47", label: "服务边界与容错设计" },
    ],
    excerpt: [
      "#### 交易服务重构",
      "",
      "负责订单、库存和支付链路的服务拆分。",
      "通过事件驱动降低核心路径耦合，并为关键调用",
      "补充超时、幂等和补偿机制。",
    ],
    explanation:
      "项目材料补充了微服务拆分、事件驱动与容错设计证据，能够支撑复杂交易系统的工程实践要求。",
  },
  {
    id: "source-notes",
    document: "03-技术总结/Go 并发编程实践.md",
    location: "L11-35",
    snippets: [
      { location: "L11-20", label: "并发控制原则" },
      { location: "L22-35", label: "内存模型与竞态排查" },
    ],
    excerpt: [
      "#### 并发控制原则",
      "",
      "对共享状态优先收敛所有权，明确 goroutine 生命周期，",
      "使用 context 管理取消信号，并通过 race detector",
      "验证关键路径中的数据竞争。",
    ],
    explanation:
      "技术总结说明候选人不只会使用并发原语，也理解生命周期、共享状态和竞态检测等底层约束。",
  },
];

const sourceFor = (
  id: string,
  document: string,
  location: string,
  label: string,
  explanation: string,
): EvidenceSource => ({
  id,
  document,
  location,
  snippets: [{ location, label }],
  excerpt: ["#### " + label, "", explanation],
  explanation,
});

export const requirementGroups: RequirementGroup[] = [
  {
    id: "technical",
    label: "技术能力",
    summaryStatus: "strong",
    summaryEvidenceCount: 3,
    requirements: [
      {
        id: "go-concurrency",
        group: "技术能力",
        text: "精通 Go 语言，理解并发模型与内存模型",
        status: "strong",
        evidenceCount: 3,
        sources: backendSources,
      },
      {
        id: "microservices",
        group: "技术能力",
        text: "熟悉微服务架构与领域驱动设计",
        status: "strong",
        evidenceCount: 2,
        sources: [
          sourceFor(
            "source-microservice",
            "02-项目经验/电商平台-交易服务.md",
            "L18-47",
            "交易服务拆分与领域边界",
            "交易服务按业务能力拆分，并通过事件驱动降低核心链路耦合。",
          ),
          sourceFor(
            "source-domain",
            "03-技术总结/领域建模复盘.md",
            "L8-26",
            "聚合边界与一致性策略",
            "资料说明了聚合边界、事务范围和最终一致性的设计取舍。",
          ),
        ],
      },
      {
        id: "postgresql",
        group: "技术能力",
        text: "熟练使用 PostgreSQL，具备索引与查询优化经验",
        status: "strong",
        evidenceCount: 2,
        sources: [
          sourceFor(
            "source-pg",
            "01-工作经历/字节跳动-后端开发工程师.md",
            "L72-84",
            "慢查询治理与索引优化",
            "通过执行计划分析、复合索引和 SQL 改写降低核心查询延迟。",
          ),
          sourceFor(
            "source-pg-project",
            "02-项目经验/报表平台.md",
            "L31-43",
            "报表查询性能治理",
            "针对大表聚合查询进行了分区与物化结果优化。",
          ),
        ],
      },
      {
        id: "redis",
        group: "技术能力",
        text: "有 Redis 使用经验，了解缓存策略与高可用方案",
        status: "partial",
        evidenceCount: 1,
        sources: [
          sourceFor(
            "source-redis",
            "02-项目经验/电商平台-交易服务.md",
            "L52-61",
            "热点数据缓存与降级",
            "项目中使用 Redis 缓存热点数据并设计了失效与降级策略，但高可用细节证据不足。",
          ),
        ],
      },
      {
        id: "kafka",
        group: "技术能力",
        text: "熟悉 Kafka 或其他消息队列的使用场景",
        status: "strong",
        evidenceCount: 2,
        sources: [
          sourceFor(
            "source-kafka",
            "02-项目经验/电商平台-交易服务.md",
            "L64-82",
            "Kafka 事件驱动链路",
            "使用 Kafka 解耦订单下游处理，并补充重试与幂等机制。",
          ),
          sourceFor(
            "source-kafka-note",
            "03-技术总结/消息可靠性.md",
            "L12-34",
            "消息可靠性治理",
            "资料涵盖重复消费、顺序、积压和死信处理。",
          ),
        ],
      },
      {
        id: "load-testing",
        group: "技术能力",
        text: "具备容量压测与性能基线建设经验",
        status: "partial",
        evidenceCount: 1,
        sources: [
          sourceFor(
            "source-load-testing",
            "04-工作记录/性能压测.md",
            "L6-19",
            "核心链路容量压测",
            "已有压测方案和结果记录，但缺少持续基线与自动回归证据。",
          ),
        ],
      },
      {
        id: "release-governance",
        group: "技术能力",
        text: "熟悉灰度发布、回滚与变更风险控制",
        status: "partial",
        evidenceCount: 1,
        sources: [
          sourceFor(
            "source-release",
            "04-工作记录/发布流程.md",
            "L10-24",
            "服务灰度与回滚流程",
            "资料确认了灰度和回滚实践，但缺少完整变更治理机制描述。",
          ),
        ],
      },
      {
        id: "capacity",
        group: "技术能力",
        text: "能够进行容量规划与系统瓶颈分析",
        status: "strong",
        evidenceCount: 2,
        sources: backendSources.slice(0, 2),
      },
      {
        id: "english",
        group: "技术能力",
        text: "能够使用英语进行技术沟通与文档协作",
        status: "partial",
        evidenceCount: 1,
        sources: [
          sourceFor(
            "source-english",
            "05-作品集/英文技术文档.md",
            "L1-18",
            "英文技术方案与评审记录",
            "已有英文文档证据，但缺少长期跨国协作场景说明。",
          ),
        ],
      },
    ],
  },
  {
    id: "engineering",
    label: "工程实践",
    summaryStatus: "partial",
    summaryEvidenceCount: 2,
    requirements: [
      {
        id: "testing",
        group: "工程实践",
        text: "具备自动化测试和持续交付实践",
        status: "partial",
        evidenceCount: 1,
        sources: [
          sourceFor(
            "source-testing",
            "01-工作经历/字节跳动-后端开发工程师.md",
            "L116-124",
            "核心链路自动化测试",
            "已有单元和集成测试证据，但持续交付流程描述不完整。",
          ),
        ],
      },
      {
        id: "observability",
        group: "工程实践",
        text: "熟悉可观测性、故障定位和稳定性治理",
        status: "strong",
        evidenceCount: 2,
        sources: [
          sourceFor(
            "source-observability",
            "01-工作经历/字节跳动-后端开发工程师.md",
            "L126-141",
            "可观测性体系建设",
            "接入指标、日志和链路追踪，并建立核心 SLO 告警。",
          ),
          sourceFor(
            "source-incident",
            "04-工作记录/稳定性复盘.md",
            "L9-22",
            "线上故障复盘",
            "复盘记录包含发现、定位、止损和长期治理动作。",
          ),
        ],
      },
      {
        id: "code-review",
        group: "工程实践",
        text: "有代码审查和工程规范建设经验",
        status: "strong",
        evidenceCount: 1,
        sources: [
          sourceFor(
            "source-review",
            "04-工作记录/团队工程规范.md",
            "L7-21",
            "代码审查与质量门禁",
            "推动团队代码审查清单和静态检查门禁落地。",
          ),
        ],
      },
      {
        id: "cloud",
        group: "工程实践",
        text: "具备云原生部署与容器编排经验",
        status: "partial",
        evidenceCount: 1,
        sources: [
          sourceFor(
            "source-cloud",
            "02-项目经验/电商平台-交易服务.md",
            "L87-96",
            "容器化部署",
            "资料确认 Docker 与 Kubernetes 使用经验，但缺少集群治理深度。",
          ),
        ],
      },
    ],
  },
  {
    id: "system-design",
    label: "系统设计",
    summaryStatus: "strong",
    summaryEvidenceCount: 2,
    requirements: [
      {
        id: "availability",
        group: "系统设计",
        text: "能设计高可用、高并发的后端系统",
        status: "strong",
        evidenceCount: 3,
        sources: backendSources,
      },
      {
        id: "tradeoffs",
        group: "系统设计",
        text: "能够解释架构权衡与演进路径",
        status: "strong",
        evidenceCount: 2,
        sources: backendSources.slice(1),
      },
      {
        id: "security",
        group: "系统设计",
        text: "理解服务安全与数据保护基础",
        status: "missing",
        evidenceCount: 0,
        sources: [],
      },
    ],
  },
  {
    id: "soft-skills",
    label: "软件素质",
    summaryStatus: "missing",
    summaryEvidenceCount: 0,
    requirements: [
      {
        id: "collaboration",
        group: "软件素质",
        text: "具备跨团队沟通与推动能力",
        status: "strong",
        evidenceCount: 2,
        sources: backendSources.slice(0, 2),
      },
      {
        id: "mentoring",
        group: "软件素质",
        text: "能够指导团队成员并沉淀方法",
        status: "missing",
        evidenceCount: 0,
        sources: [],
      },
    ],
  },
  {
    id: "bonus",
    label: "加分项",
    summaryStatus: "strong",
    summaryEvidenceCount: 1,
    requirements: [
      {
        id: "open-source",
        group: "加分项",
        text: "有开源项目或技术社区贡献",
        status: "strong",
        evidenceCount: 1,
        sources: [
          sourceFor(
            "source-open-source",
            "05-作品集/开源项目.md",
            "L4-18",
            "开源项目维护记录",
            "资料记录了开源项目的功能迭代、Issue 处理和文档维护。",
          ),
        ],
      },
    ],
  },
];

export const allRequirements = requirementGroups.flatMap(
  (group) => group.requirements,
);
