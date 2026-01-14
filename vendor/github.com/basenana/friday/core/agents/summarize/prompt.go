package summarize

const (
	DEFAULT_SUMMARIZE_PROMPT = `<background>
Your job is to summarize a history of previous messages in a conversation between an AI persona and a human.
The conversation you are given is a from a fixed context window and may not be complete.
Messages sent by the AI are marked with the 'assistant' role.
The AI 'assistant' can also make calls to tools, whose outputs can be seen in messages with the 'tool' role.
Things the AI says in the message content are considered inner monologue and are not seen by the user.
The only AI messages seen by the user are from when the AI uses 'send_message'.
Messages the user sends are in the 'user' role.
The 'user' role is also used for important system events, such as login events and heartbeat events (heartbeats run the AI's program without user action, allowing the AI to act without prompting from the user sending them a message).
Summarize what happened in the conversation from the perspective of the AI (use the first person from the perspective of the AI).
Keep your summary less than 1000 words, do NOT exceed this word limit.
Only output the summary, do NOT include anything else in your output, and use the same language as the user input content.
</background>

<summary_core_objective>
- Based on historical messages, summarize and generate text that conforms to the definition in summary_formatting.
- Do not output any content other than the summary text.
</summary_core_objective>


<summary_formatting>
The summary should refer to the template below:

## Basic Information

Participants: Individuals/roles involved in the dialogue
Topic/Purpose: The core issue or goal of the dialogue

## Key Content Extraction

Main Points: Core opinions expressed by each party, preliminary conclusions, and current progress
Points of contention: Existing disagreements or issues for which consensus has not been reached
Action Items: The next steps that have been confirmed but have not yet been implemented
Important Data: Key figures, dates, names, and other hard information involved

## Additional Information

Special Context: Context that may affect understanding (e.g., preconditions, urgency)
Emotional Labeling: Label the emotional state of participants if necessary (e.g., "User expresses dissatisfaction")
</summary_formatting>

<citation_requirements>
- Always cite sources using markdown footnote format (e.g., [^1])
- List all referenced URLs at the end of your response
- Clearly distinguish between quoted information and your own analysis
- Respond in the same language as the user's query

  <citation_examples>
    <example>
    According to recent studies, global temperatures have risen by 1.1°C since pre-industrial times[^1].

    [^1]: [Climate Report in 2023](https://example.org/climate-report-2023)
    </example>
    <example>
    以上信息主要基于业内测评和公开发布会（例如2025年4月16日的发布内容）的报道，详细介绍了 O3 与 O4-mini 模型在多模态推理、工具使用、模拟推理和成本效益等方面的综合提升。[^1][^2]

    [^1]: [OpenAI发布o3与o4-mini，性能爆表，可用图像思考](https://zhuanlan.zhihu.com/p/1896105931709849860)
    [^2]: [OpenAI发新模型o3和o4-mini！首次实现"图像思维"（华尔街见闻）](https://wallstreetcn.com/articles/3745356)
    </example>
  </citation_examples>
</citation_requirements>
`

	DEFAULT_USER_MESSAGE = "Please summarize the historical messages as required, from now on, every character you output will become part of the abstract"
)
