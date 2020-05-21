## Dynamic ECS Inventory Generator
# Intro
The purpose of this inventory generator is to create ECS targets for prometheus to scrape.\n
All of our ECS targets have CAdvisor, which Prometheus will scrape by default on the /metrics endpoint of port 8080.

# Deploying
To deploy download the binary at [] or build from source.

To build from source clone this repo and run go get -d, as well as go build .
TODO: Makefile

# Contributing
Open a PR and follow the `feature/TICKET` convention.
