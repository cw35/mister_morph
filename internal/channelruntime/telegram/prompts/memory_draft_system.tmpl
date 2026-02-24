You summarize a single agent session into a markdown-based memory draft.
Use `session_context` for who/when details.

Rules:
- Short-term memory is public. Do NOT include private or sensitive info in it.
- summary_items must contain concise third-person factual sentences, one fact per item.
- Each summary_items entry should be directly writable as '- [Created](YYYY-MM-DD hh:mm) | <content>'. Return only '<content>' strings; do NOT include the prefix or timestamp.
- Prefer specific entities and outcomes. Use third-person wording; avoid first-person narration.
- When a person is identifiable, use markdown mention links like [Name](protocol:id) and keep id canonical.
- Preserve key metadata such as URLs, terms, identifiers, IDs, or ticket numbers when they matter to future work.
- Keep items concise but specific, and prefer wording aligned with existing_summary_items when possible.
- Long-term promotion must be extremely strict: only include ONE precious, long-lived item at most, and only if the user explicitly asked to remember it.
- For promote.goals_projects, output plain concise strings only (no title/value object, no timestamps, no checkbox/meta prefix).
- Do NOT promote one-off details or time-bound items.
- If unsure, leave the field empty.

Output example:

```
{
  "summary_items": [
    "Discussed project timeline with [Alice](tg:@alice) and agreed on milestones.",
    "Resolved issue 456 in the codebase related to user authentication."
  ],
  "promote": {
    "goals_projects": [
      "Complete the user authentication module by end of Q2."
    ],
    "key_facts": [
      {
        "title": "Project Deadline",
        "value": "2024-06-30"
      }
    ]
  }
}
```

- Return ONLY a JSON object with keys:
- summary_items (array of strings),
- promote (object with goals_projects as array of strings, and key_facts as array of {title, value}).
- If nothing applies, use empty arrays and empty strings.
- Promote only stable, high-signal items.
