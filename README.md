# Acai Travel AI Assistant

A conversational AI assistant backend built in Go, powered by OpenAI and MongoDB. Users can start and continue conversations, and the assistant can answer travel-related questions using real-time tools.

---

## API Flows

All endpoints are served via Twirp over HTTP POST at:
```
http://localhost:8080/twirp/acai.chat.ChatService/<MethodName>
```

---

### 1. StartConversation

**POST** `/twirp/acai.chat.ChatService/StartConversation`

**Request**
```json
{ "message": "What is the weather in Barcelona?" }
```

**Flow**
```
Request arrives
      │
      ▼
Validate message (empty = error)
      │
      ├─────────────────────────┐
      ▼                         ▼
Title() → OpenAI          Reply() → OpenAI + Tools
(gpt-4o-mini, 20 tokens)  (gpt-4.1, 500 tokens)
      │                         │
      └──────────┬──────────────┘
                 ▼
         Both results ready
                 │
                 ▼
         Append assistant message
                 │
                 ▼
         Save to MongoDB (background)
                 │
                 ▼
         Return response immediately
```

**Response**
```json
{
  "conversation_id": "abc123",
  "title": "Weather in Barcelona",
  "reply": "The weather in Barcelona is currently 24°C and sunny."
}
```

---

### 2. ContinueConversation

**POST** `/twirp/acai.chat.ChatService/ContinueConversation`

**Request**
```json
{
  "conversation_id": "abc123",
  "message": "What about tomorrow?"
}
```

**Flow**
```
Request arrives
      │
      ▼
Validate conversation_id + message
      │
      ▼
Load conversation from MongoDB
      │
      ▼
Append new user message
      │
      ▼
Reply() → OpenAI (with full message history for context)
      │
      ▼
Tool loop (up to N iterations)
  ├── AI calls tool? → execute → loop again
  └── AI gives answer? → done
      │
      ▼
Append assistant reply
      │
      ▼
Update conversation in MongoDB
      │
      ▼
Return reply
```

**Response**
```json
{
  "reply": "Tomorrow will be partly cloudy with a high of 22°C."
}
```

---

### 3. ListConversations

**POST** `/twirp/acai.chat.ChatService/ListConversations`

**Request**
```json
{}
```

**Flow**
```
Request arrives
      │
      ▼
Fetch all conversations from MongoDB
(sorted by newest first, messages excluded)
      │
      ▼
Return list
```

**Response**
```json
{
  "conversations": [
    { "id": "abc123", "title": "Weather in Barcelona", "timestamp": "2026-06-03T10:00:00Z" },
    { "id": "def456", "title": "Public Holidays in Japan", "timestamp": "2026-06-02T09:00:00Z" }
  ]
}
```

---

### 4. DescribeConversation

**POST** `/twirp/acai.chat.ChatService/DescribeConversation`

**Request**
```json
{ "conversation_id": "abc123" }
```

**Flow**
```
Request arrives
      │
      ▼
Validate conversation_id
      │
      ▼
Fetch conversation from MongoDB by ID
      │
      ├── Not found → return 404 error
      └── Found → return full conversation with all messages
```

**Response**
```json
{
  "conversation": {
    "id": "abc123",
    "title": "Weather in Barcelona",
    "timestamp": "2026-06-03T10:00:00Z",
    "messages": [
      { "id": "m1", "role": "USER", "content": "What is the weather in Barcelona?", "timestamp": "..." },
      { "id": "m2", "role": "ASSISTANT", "content": "The weather is 24°C and sunny.", "timestamp": "..." }
    ]
  }
}
```

---

### Tool Call Flow (inside Reply)

When the AI needs real-time information it calls tools before answering:

```
Reply() called
      │
      ▼
Send messages to OpenAI
      │
      ├── AI returns tool call?
      │         │
      │         ▼
      │   Execute tool
      │   ├── get_today_date   → returns current date/time
      │   ├── get_weather      → calls WeatherAPI (current or forecast)
      │   ├── get_holidays     → loads iCal calendar, filters by date
      │   └── get_country_info → calls restcountries.com
      │         │
      │         ▼
      │   Append tool result → loop back to OpenAI
      │
      └── AI returns text answer?
                │
                ▼
          Return final reply
```

---

## Changes Summary

### Title Bug Fix

The original `Title()` method had two bugs. First, the instruction was set as an `AssistantMessage` instead of a `SystemMessage`, so the AI treated it as something it had already said rather than an instruction to follow. Second, the for loop was overwriting index 0 of the message slice, causing the instruction to be lost entirely. The fix places the instruction as a `SystemMessage` first, then appends user messages after it using `append` so nothing is overwritten. The model was also changed from `o1` (a slow reasoning model) to `gpt-4o-mini` with a 20 token limit, which is fast and cheap for simple title generation.

---

### Performance Optimizations

`Title()` and `Reply()` are independent — neither needs the other's result. They now run in parallel using Go goroutines and channels, so the total wait time equals the slower of the two rather than the sum of both. The MongoDB save is moved to a background goroutine so the user gets their response immediately without waiting for the database write. Tool definitions and system messages are built once at startup and stored in the Assistant struct, avoiding repeated object creation on every request. Message slices are pre-allocated with known capacity to avoid memory reallocation. The tool call iteration limit is calculated dynamically as `(numTools × 2) + 1` so it scales automatically as tools are added.

---

### Tools Refactoring with Strategy Pattern

Originally all tool definitions and their logic were mixed together inside one large `Reply()` function. Adding a new tool required editing `Reply()` in three places: the tool definition list, the switch statement, and the handler logic. The refactor introduces a `Tool` interface with three methods — `Name()`, `Definition()`, and `Execute()`. Each tool is now its own file in the `internal/chat/assistant/tool/` package. Adding a new tool means creating one new file and registering it with one line in `New()`. The `Reply()` function never needs to change when tools are added or removed.

```
Tool interface
├── tool_date.go      → get_today_date
├── tool_weather.go   → get_weather (current + forecast)
├── tool_holidays.go  → get_holidays (multi-country, date filtering)
└── tool_country.go   → get_country_info
```

---

### Country Info Tool

A new tool was added using the free `restcountries.com` API — no API key required. It answers common travel questions about any of 250+ countries. The tool accepts a country name and returns capital city, official language, currency with symbol, population, region, timezones, and bordering countries. The AI automatically calls this tool when users ask about visas, money, language, or geography for a specific country.

Example questions it handles:
- "What currency does Japan use?"
- "What language do they speak in Morocco?"
- "What countries border India?"
- "I am planning to visit Thailand, what do I need to know?"

---

### Instrumentation

The server includes OpenTelemetry instrumentation with the service name `tech-challenge`. Every HTTP request is logged with method, path, and status code via the Logger middleware. Errors are logged at ERROR level and successful requests at INFO level. The Recovery middleware catches any unexpected panics and returns a 500 response instead of crashing the server. All assistant operations log the conversation ID for traceability, and tool calls log the tool name and arguments so you can see exactly what the AI requested.

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `OPENAI_API_KEY` | Yes | OpenAI API key |
| `WEATHER_API_KEY` | Yes | WeatherAPI.com key |
| `MONGODB_URI` | No | MongoDB connection string (default: `mongodb://acai:travel@localhost:27017`) |
| `HOLIDAY_CALENDAR_LINK` | No | Custom iCal URL (default: Catalonia holidays) |

---

## Author

Sohil Mansuri

