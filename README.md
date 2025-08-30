# Docker Telegram Bot

This is a claude coded golang application that connects to
[telegram](https://core.telegram.org/bots/api) and provides a chat interface
to your local docker engine. 

There is no security controls except the user ID provided.

## Telegram Setup Instructions:

Create Telegram Bot:

* Message @BotFather on Telegram
* Use /newbot and follow instructions
* Save the bot token

Get Your User ID:

* Message @userinfobot on Telegram to get your user ID

## Running the bot

Export the following ENV variables

* TOKEN - (The above bot token)
* USER_ID - (Your user id from telegram)

```
$ TOKEN=XXXXX:XXXX USER_ID=XXXXX ./docker-telegram-bot
12:48PM INF Docker Telegram Bot started allowed_user=190000000 bot_username=ElvinDockerBot component=docker-telegram-bot
12:48PM INF Processing command command=/list component=docker-telegram-bot user_id=190000000
```

You can build the docker image with the provided [Dockerfile](./Dockerfile) and run it with the docker socket mounted. (Or just use visago/docker-telegram-bot:latest)

```
docker run -d -e TOKEN=XXXXX:XXXX -e USER_ID=XXXXX -v /var/run/docker.sock:/var/run/docker.sock visago/docker-telegram-bot:latest
```

There is a sample [docker-compose.yaml](docker-compose.yaml) file to use too

## Using the bot

* /list - List all containers
* /detailed - List all containers with extra details
* /start <name> - Start a container
* /stop <name> - Stop a container
* /restart <name> - Restart a container
* /logs <name> [lines] - Show container logs (default: 10 lines, max: 1000)
* /help - Show this help message
