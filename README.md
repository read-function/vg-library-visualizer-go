# Video Game Library Visualizer (Go edition)

A desktop app to display your video game library. Displays a cover on the foreground, and
blurred screenshots or artwork in the background. Connects to your Steam account then
fetches images from IGDB.

In attempt to learn Go I created this project to try out a variety of common things done
in applications. I don't really intend to keep this up to date as this was done as a
learning exercise. 

## Configuration
This application has two configuration files. One for application behavior, and another
for holding personal account info. To run the app you'll need to create API accounts with
Steam and IGDB (Twitch). I might add something for itch.io eventually.

In the config.properties file you can change how long the game cover stays in place and how
many background transitions to go through during that time.

In the config-secret.properties (not included in the repo) you place your credentials as
described in the following sections.

### Fetching your Steam Id and API Key
Go edit your Steam profile and in the General section there is the possibility to change
your URL. If you have not changed the custom URL, Steam will show your profile URL. Use the 
number you see in the URL and set in the value `steam.client.id=` in the 
config-secret.properties file. 

You'll also need to create an API key. Fill out the form here https://steamcommunity.com/dev/apikey
and put the key in the `steam.client.key=` value in the config-secret.properties file.

### Fetch IGDB Credentials
Follow the Account Creation instruction here https://api-docs.igdb.com/#about and put the 
client id in `igdb.client.id=` and the secret in `igdb.client.secret=` in the 
config-secret.properties file.