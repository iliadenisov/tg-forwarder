# Telegram Channel Messages Forwarder

Forwards new messages from one Telegram channel to another.
Based on [gotd](https://github.com/gotd/td) library.

## Description

Being logged in as regular user account, application listens to new channel
message updates and forwards them from source to destination channel omitting
original post's author name. Forward operation is delayed for a 5 seconds
in order to allow all heavy media to be uploaded by source party.

It's required to use Telegram user account credentials, no surprise here:
bots are not allowed to join channels themselves.

Telegram's API `api_id` and `api_hash` must be
[obtained](https://core.telegram.org/api/obtaining_api_id)
in order to configure application.

## Configuration parameters

Application expects following environment variables to be set:

| Name | Description
| -    | -
| `APP_ID`           | [app_id](https://my.telegram.org/) for Telegram app.
| `APP_HASH`         | [app_hash](https://my.telegram.org/) for Telegram app.
| `APP_ENV`          | Set to `development` to enable debug-level logs.
| `ACCOUNT_PHONE`    | Phone number of Telegram user account.
| `ACCOUNT_PASSWORD` | Password for Telegram user account.
| `SESSION_FILE`     | Persistent data storage for Telegram client.
| `FORWARD_MAP`      | Channel forwarding mapping (see below).

## Telegram user authentication

First started, app will eventually request Telegram Auth Code from STDIN.
Running as Docker Container, first run command might be the following:

```shell
docker run -it --env-file app.env --name ${CONTAINER_NAME} ${IMAGE} \
    /bin/sh /home/app/entrypoint.sh /usr/bin/forwarder
```

Then just exit the container and run in normal mode with `docker start`.

After successfull authentication, session will be kept in `$SESSION_FILE` file.
It's recommended to map this path outside of Docker container if you're
not a fan of logging in to the user's account each time application is started.

## Forwarding rules

Parameter `FORWARD_MAP` is expected to be set in following notation:

```text
FORWARD_MAP=<dst>:<src>,<src>...
```

Where `dst` is destination channel id, `src` is source channel id(s) to monitor
for a new messages for that destination.
Multiple source channels are comma-separated.
Multiple destination mappings are pipe-separated.
User's account required to have member access to source channels
and have admin priveleges for destination channels.

Examples:

```bash
# forward from channel_id=123 to channel_id=666
export FORWARD_MAP=666:123

# forward from channel_id=123 and channel_id=456 to channel_id=666
export FORWARD_MAP=666:123,456

# as well, forward from channel_id=321 and channel_id=654 to channel_id=999
export FORWARD_MAP=666:123,456|999:321,654
```

If you have doubts of providing channel ids, omit `FORWARD_MAP` parameter.
Once being authenicated, application prints available channels and exits.

## Known issues

- Underlying [gotd](https://github.com/gotd/td) library sometimes stops sending
  updates for new messages in channels. It is likely a MTProto issue because
  I've seen enough "stuck" channel updates on Desktop/Mobile clients as well.
