package domain

type ResumePackagingStrategy struct {
	ID               string
	Level            float64
	Label            string
	Description      string
	EvidenceLimit    int
	LanguageStrength string
	SelectionPolicy  string
	InferencePolicy  string
	Guardrails       []string
}

func ResumePackagingStrategyForLevel(
	level float64,
) (ResumePackagingStrategy, bool) {
	switch level {
	case 0:
		return ResumePackagingStrategy{
			ID:               "conservative",
			Level:            0,
			Label:            "保守",
			Description:      "只使用明确事实，主要做选择、排序、压缩和清晰化。",
			EvidenceLimit:    4,
			LanguageStrength: "克制、具体，避免放大职责和影响。",
			SelectionPolicy:  "优先选择与 JD 直接相关且来源最清楚的事实。",
			InferencePolicy:  "只允许从来源事实做低风险归纳。",
			Guardrails: []string{
				"不新增职责、技术、客户或结果。",
				"不写入未经确认的数字。",
			},
		}, true
	case 0.5:
		return ResumePackagingStrategy{
			ID:               "balanced",
			Level:            0.5,
			Label:            "平衡",
			Description:      "默认档，允许从事实中归纳能力和业务价值。",
			EvidenceLimit:    6,
			LanguageStrength: "清晰、有重点，突出岗位相关能力。",
			SelectionPolicy:  "兼顾直接证据、职责覆盖和 JD 关键词。",
			InferencePolicy:  "允许解释已有工作的业务价值，但不新增事实。",
			Guardrails: []string{
				"不新增职责、技术或结果。",
				"不把团队成果改写成个人独立成果。",
			},
		}, true
	case 1:
		return ResumePackagingStrategy{
			ID:               "amplified",
			Level:            1,
			Label:            "强化",
			Description:      "使用更有竞争力的职业定位和语言力度。",
			EvidenceLimit:    8,
			LanguageStrength: "更主动、更结果导向，但仍保持事实边界。",
			SelectionPolicy:  "扩大相关经历权重，优先呈现可解释亮点。",
			InferencePolicy:  "允许更强的职业定位，不允许新增经历或数字。",
			Guardrails: []string{
				"不得声称用户没有承担过的职责。",
				"不得声称用户没有使用过的技术。",
				"不得自动加入未经确认的具体数字。",
			},
		}, true
	default:
		return ResumePackagingStrategy{}, false
	}
}
