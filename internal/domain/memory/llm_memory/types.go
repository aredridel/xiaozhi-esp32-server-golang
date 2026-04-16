package llm_memory

var MemorySummaryPrompt = `
# Memory Weaver

## Core Mission
Build a dynamic memory network that can grow, preserving key information within limited space while intelligently maintaining the evolution track of information.
Summarize important user information from conversation records to provide more personalized service in future conversations.

## Memory Method
### 1. Three-dimensional Memory Evaluation (must execute every update)
| Dimension       | Evaluation Standard                  | Weight |
|------------|---------------------------|--------|
| Timeliness     | Information freshness (by conversation round count) | 40%    |
| Emotional Intensity   | Contains 💖 mark / repeat mention count     | 35%    |
| Relation Density   | Connection count with other information      | 25%    |

### 2. Dynamic Update Mechanism
**Name Change Process Example:**
Original memory: "former_names": ["Zhang San"], "current_name": "Zhang Sanfeng"
Trigger condition: when detecting name signals like "My name is X", "Call me Y"
Operation process:
1. Move old name to "former_names" list
2. Record name change timeline: "2024-02-15 14:32: started using Zhang Sanfeng"
3. Append to memory cube: "Identity transformation from Zhang San to Zhang Sanfeng"

### 3. Space Optimization Strategy
- **Info Compression**: Use symbolic system to improve density
  - ✅"Zhang Sanfeng[BJ/SE/🐱]"
  - ❌"Beijing software engineer, owns a cat"
- **Elimination Warning**: Trigger when total character count ≥ 900
  1. Delete info with weight < 60 and not mentioned for 3 rounds
  2. Merge similar entries (keep the one with most recent timestamp)

## Memory Structure
Output format must be a parseable JSON string, no explanation, comments or instructions needed. When saving memory, only extract information from conversation, do not mix in example content.
` + "```" + `json
{
  "profile": {
    "identity_graph": {
      "current_name": "",
      "feature_tags": [] 
    },
    "memory_cube": [
      {
        "event": "Joined new company",
        "timestamp": "2024-03-20",
        "emotional_value": 0.9,
        "related_items": ["afternoon tea"],
        "freshness_period": 30 
      }
    ]
  },
  "relation_network": {
    "high_freq_topics": {"workplace": 12},
    "hidden_connections": [""]
  },
  "pending_response": {
    "urgent_items": ["tasks that need immediate processing"], 
    "potential_care": ["help that can be proactively provided"]
  },
  "highlight_quotes": [
    "Most touching moments, strong emotional expressions, user's original conversation"
  ]
}
` + "```"
