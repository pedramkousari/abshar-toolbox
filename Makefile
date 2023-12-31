build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/abshar-toolbox main.go

build-server: build
	scp bin/abshar-toolbox root@10.10.10.217:/var/www/abshar/bin


build-server-mj: build
	scp bin/abshar-toolbox root@10.10.10.207:/var/www/html/baadbaan-docker

build-232: build
	scp bin/abshar-toolbox root@10.10.10.232:/var/www/html/update-toolbox

create:
	go run main.go patch create ./package.json

run:
	go run main.go serve

update:
	go run main.go patch update ./builds/12.tar.gz.enc