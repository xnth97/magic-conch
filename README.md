# magic-conch

> Why don't you ask the magic conch?

magic-conch is a Telegram bot powered by Azure Cognitive Services.

## Features

- Integrates with ChatGPT using Azure Cognitive Services. **No OpenAI API support**, as there are too many equivalents.
- Support both one-to-one chat and group chat. For group chat, must use `/chat` to trigger magic conch's response.
- Use `/reset` to reset the conversation history. Each chat maintains its own conversation context.

## Usage

1. Clone this repo

2. Copy `EXAMPLE_config.json` as `config.json` and fill in with your own information. You must have a valid Azure model deployment. Please see __Prerequisite__ section of [Azure documentation](https://learn.microsoft.com/en-us/azure/cognitive-services/openai/chatgpt-quickstart?tabs=command-line&pivots=rest-api). An example is like:

```js
{
    // This value can be found in the Keys & Endpoint section when examining your resource from the Azure portal. Alternatively, you can find the value in the Azure OpenAI Studio > Playground > Code View
    "base_url": "https://EXAMPLE.openai.azure.com",
    // You will need to replace gpt-35-turbo with the deployment name you chose when you deployed the ChatGPT or GPT-4 models.
    "model": "gpt-35-turbo",
    // API version of the model, see on Azure docs.
    "api_version": "2023-03-15-preview",
    // This value can be found in the Keys & Endpoint section when examining your resource from the Azure portal. You can use either KEY1 or KEY2.
    "api_key": "AZURE_API_KEY_HERE",
    // Telegram Bot API token.
    "telegram_api_key": "TELEGRAM_API_KEY_HERE",
    // Telegram chat ID numbers that you want to have access to this bot. Left empty ([]) if you don't want any limitation.
    "allowed_chat_ids": [],
    // How many messages can be included in one conversation. The more messages included, the better ChatGPT understands the context, however also more tokens it consumes.
    "past_messages_included": 10,
    // Max tokens can be used.
    "max_tokens": 800
}
```

3. Run with `go run .` or build an executable `go build .`

## License

magic-conch is available under MIT license. See the [LICENSE](LICENSE) file for more info.
