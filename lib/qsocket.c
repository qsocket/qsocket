#define _GNU_SOURCE // LD_PRELOAD=$PWD/qsocket.so nc google.com 80

#include "md5.h"
#include <arpa/inet.h>
#include <dlfcn.h>
#include <errno.h>
#include <netdb.h>
#include <netinet/in.h>
#include <stdio.h>
#include <stdlib.h> /* getenv */
#include <string.h>
#include <sys/socket.h>
#include <unistd.h>

#if __amd64__ || __amd64 || __x86_64__ || __x86_64 || _M_AMD64 || _M_X64
#define ARCH_TAG ((unsigned char)0xE0)
#elif i386 || __i386 || __i386__
#define ARCH_TAG ((unsigned char)0x20)
#elif __aarch64__
#define ARCH_TAG ((unsigned char)0x40)
#elif __arm__ || _M_ARM
#define ARCH_TAG ((unsigned char)0x60)
#elif __mips__ || mips || __MIPS__
#define ARCH_TAG ((unsigned char)0xA0)
#else
#define ARCH_TAG ((unsigned char)0x00)
#endif

#if __unix__ || __LINUX__
#define OS_TAG ((unsigned char)0x1C)
#elif __APPLE__
#define OS_TAG ((unsigned char)0x04)
#elif _WIN32 || _WIN64
#define OS_TAG ((unsigned char)0x08)
#elif __ANDROID__
#define OS_TAG ((unsigned char)0x0C)
#elif __FreeBSD__
#define OS_TAG ((unsigned char)0x14)
#elif BSD
#define OS_TAG ((unsigned char)0x18)
#else
#define OS_TAG ((unsigned char)0x00)
#endif

#define PEER_ID_CLIENT 0x01
#define PEER_ID_PROXY 0x02

#ifdef DEBUG
#define DEBUG_PRINT(fmt, args...) fprintf(stderr, fmt, ##args)
#else
#define DEBUG_PRINT(fmt, args...) /* Don't do anything in release builds */
#endif

// Magic bytes
#define KNOCK_PREAMBLE "\xC0\xDE"
// Env. vars.
#define QSOCKET_GATE "gate.qsocket.io"
#define QSOCKET_SECRET_VAR "QSOCKET_SECRET"
#define KNOCK_CHECKSUM_BASE 0xEE
// Error codes.
#define ERR_DLSYM_FAILED 0xF0   // dlsym call failed.
#define ERR_INVALID_GATE 0xF1   // Invalid qsocket gate IP value.
#define ERR_INVALID_SECRET 0xF2 // Invalid qsocket secret
#define ERR_INVALID_TAG 0xF3    // Invalid qsocket tag value
#define ERR_MALLOC_FAILED 0xF4  // Malloc failed.
#define ERR_KNOCK_FAILED 0xF4   // Knock sequence failed.
#define KNOCK_SUCCESS 0xE0
#define KNOCK_FAIL 0xE1
#define KNOCK_BUSY 0xE2

typedef int (*connect_t)(int sockfd, const struct sockaddr *addr,
                         socklen_t addrlen);
connect_t real_connect;
typedef struct hostent *(*gethostbyname_t)(const char *name);
gethostbyname_t real_gethostbyname;
unsigned char secret[MD5_DIGEST_LENGTH];

unsigned char cacl_checksum(unsigned char *data, int len) {
  unsigned int sum = 0;
  for (int i = 0; i < len; i++) {
    sum += (((unsigned char)data[i] << 2) % KNOCK_CHECKSUM_BASE);
  }
  return (unsigned char)(sum % KNOCK_CHECKSUM_BASE);
}

unsigned char *get_qsocket_secret() {
  char *var = getenv(QSOCKET_SECRET_VAR);
  if (var == NULL) {
    exit(ERR_INVALID_SECRET);
  }
  return md5(var, strlen(var));
}

void *get_qsocket_gate_ip(char *ip) {
  struct hostent *hent;
  struct in_addr **addr_list;
  hent = real_gethostbyname(QSOCKET_GATE);
  addr_list = (struct in_addr **)hent->h_addr_list;
  DEBUG_PRINT("[*] Found %d IPs\n", sizeof(addr_list));
  if (addr_list[0] != NULL) {
    strcpy(ip, inet_ntoa(*addr_list[0]));
    DEBUG_PRINT("[*] IP[0] = %s\n", ip);
    return ip;
  }
  return NULL;
}

struct hostent *gethostbyname(const char *name) {
  DEBUG_PRINT("[*] Called gethostbyname(%s) ...\n", name);
  if (!real_gethostbyname) {
    const char *err;
    real_gethostbyname = (gethostbyname_t)dlsym(RTLD_NEXT, "gethostbyname");
    if ((err = dlerror()) != NULL) {
      exit(ERR_DLSYM_FAILED);
    }
  }

  if (!strcmp(name, "qsocket")) {
    return real_gethostbyname(QSOCKET_GATE);
  }
  return real_gethostbyname(name);
}

int connect(int sockfd, const struct sockaddr *addr, socklen_t addrlen) {
  if (!real_connect) {
    const char *err;
    real_connect = (connect_t)dlsym(RTLD_NEXT, "connect");
    if ((err = dlerror()) != NULL) {
      exit(ERR_DLSYM_FAILED);
    }
  }
  char addr_str[INET_ADDRSTRLEN];
  inet_ntop(addr->sa_family, &(((struct sockaddr_in *)addr)->sin_addr),
            addr_str, INET_ADDRSTRLEN);
  DEBUG_PRINT("[>] Called connect(%d,%s)\n", sockfd, addr_str);
  if (addr->sa_family != AF_INET && addr->sa_family != AF_INET6) {
    return real_connect(sockfd, addr, addrlen);
  }

  char ip[40];
  if (get_qsocket_gate_ip(ip) != NULL && strcmp(ip, addr_str)) {
    return real_connect(sockfd, addr, addrlen);
  }

  memset((void *)&addr->sa_data[0], 0x00, 1); // manually set port number to 80
  memset((void *)&addr->sa_data[1], 0x50, 1); // ...

  int ret;
  while (1) {
    ret = real_connect(sockfd, addr, addrlen);
    if (ret == 0) {
      // const char* knock =
      // "\xC0\xDE\x58\x41\x41\x41\x41\x41\x41\x41\x41\x41\x41\x41\x41\x41\x41\x41\x41\x01";
      unsigned char *knock = NULL;
      knock = malloc(20);
      if (!knock) { /* validate memory was allocated -- every time */
        exit(ERR_MALLOC_FAILED);
      }
      DEBUG_PRINT("[*] Calculating checksum...\n");
      unsigned char *secret = get_qsocket_secret();
      DEBUG_PRINT("[*] Secret: ");
      for (int i = 0; i < MD5_DIGEST_LENGTH; i++)
        DEBUG_PRINT("%02x", secret[i]);
      DEBUG_PRINT("\n");
      unsigned char checksum = cacl_checksum(secret, MD5_DIGEST_LENGTH);
      DEBUG_PRINT("[*] Checksum: 0x%02x\n", checksum);
      memcpy(knock, KNOCK_PREAMBLE, 2);
      memcpy(&knock[2], &checksum, 1);
      memcpy(&knock[3], secret, MD5_DIGEST_LENGTH);
      knock[19] = (OS_TAG | ARCH_TAG | PEER_ID_CLIENT | PEER_ID_PROXY);
      // memcpy(&knock[19], &tag, 1);

      ssize_t n = write(sockfd, knock, 20);
      if (n == 20) {
        unsigned char resp;
        n = read(sockfd, (void *)&resp, 1);
        if (n == 1 && resp == KNOCK_SUCCESS) {
          // do nothing
        } else if (n == 1 && resp == KNOCK_BUSY) {
          DEBUG_PRINT("[-] Socket busy!\n");
        } else if (n == 1 && resp == KNOCK_FAIL) {
          DEBUG_PRINT("[-] Connection refused (no server listening with given "
                      "secret)\n");
        } else {
          DEBUG_PRINT("[-] Knock sequence failed!\n");
        }
        // fprintf(stdout, "knock successfull!\n");
      } else {
        exit(ERR_KNOCK_FAILED);
      }
      break;
    } else {
      if (errno != 115 && errno != 114) {
        break;
      }
    }
  }

  return ret;
}
__attribute__((constructor)) static void setup(void) {
  DEBUG_PRINT("[>] Called setup()\n");
}
