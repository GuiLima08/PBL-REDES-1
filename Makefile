build/server:
	docker build -t bdaemonis/aircraft-server:latest -f Dockerfile.server .

build/user:
	docker build -t bdaemonis/aircraft-user:latest -f Dockerfile.user .

build/anemo:
	docker build -t bdaemonis/aircraft-anemo:latest -f Dockerfile.anemo .

build/fuel:
	docker build -t bdaemonis/aircraft-fuel:latest -f Dockerfile.fuel .

build/siren:
	docker build -t bdaemonis/aircraft-siren:latest -f Dockerfile.siren .

build: build/server build/user build/anemo build/fuel build/siren

push/server:
	docker push bdaemonis/aircraft-server:latest

push/user:
	docker push bdaemonis/aircraft-user:latest

push/anemo:
	docker push bdaemonis/aircraft-anemo:latest

push/fuel:
	docker push bdaemonis/aircraft-fuel:latest

push/siren:
	docker push bdaemonis/aircraft-siren:latest

push: push/server push/user push/anemo push/fuel push/siren

clean/containers:
	docker container prune -f

clean/images:
	docker image prune -a -f

clean: clean/containers clean/images

IP ?= host.docker.internal
PORT ?= 8080

up/server:
	docker run -d --name server -p $(PORT):$(PORT)/tcp -p $(PORT):$(PORT)/udp bdaemonis/aircraft-server:latest

up/user:
	docker run -it bdaemonis/aircraft-user:latest $(IP) $(PORT)

up/anemo:
	docker run -d bdaemonis/aircraft-anemo:latest $(IP) $(PORT)

up/fuel:
	docker run -d bdaemonis/aircraft-fuel:latest $(IP) $(PORT)

up/siren:
	docker run -d bdaemonis/aircraft-siren:latest $(IP) $(PORT)

up: up/server up/siren up/anemo up/fuel up/user

stop:
	docker stop $$(docker ps -aq)
