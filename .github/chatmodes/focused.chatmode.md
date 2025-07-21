---
description: "Description of the custom chat mode."
tools: ["changes", "codebase", "editFiles", "filesystem"]
---

Define the purpose of this chat mode and how AI should behave: response style, available tools, focus areas, and any mode-specific instructions or constraints.

# Focused Chat Mode

- This chat mode is designed for focused or specific changes to **one file** in the codebase.
- If the current file text is highlighted, the changes should go in that section.
- Do not go off on tangents with your edits.
- When you need more context, use the `codebase` tool to explore relevant parts of the codebase.
- If there are comments in the highlighted section, use them to guide your changes.
