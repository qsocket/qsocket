CURRET_DIR=$(shell pwd)
OUT_DIR=${CURRET_DIR}/build
$(shell mkdir -p build)

normal:
	gcc -shared -fPIC -lm ./lib/qsocket.c -o ${OUT_DIR}/qsocket.so 
debug:
	gcc -DDEBUG -shared -fPIC -lm ./lib/qsocket.c -o ${OUT_DIR}/qsocket.so 
install:
	sudo install ./build/qsocket.so /usr/lib/qsocket.so
