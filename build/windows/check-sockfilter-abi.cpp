// Compile-only check; does not load the SDK or driver.
// g++ -std=c++11 -fsyntax-only build/windows/check-sockfilter-abi.cpp
#include <winsock2.h>
#include <windows.h>
#include <cstddef>
#define _C_API
#include "../../nfsdk/include/nfevents.h"
static_assert(sizeof(void*) == 8, "x64 only");
static_assert(sizeof(NF_RULE_EX) == 643, "rule size");
static_assert(offsetof(NF_RULE_EX, filteringFlag) == 79, "rule flag offset");
static_assert(offsetof(NF_RULE_EX, processName) == 83, "process mask offset");
static_assert(sizeof(NF_TCP_CONN_INFO) == 67, "TCP info size");
static_assert(offsetof(NF_TCP_CONN_INFO, localAddress) == 11, "local address offset");
static_assert(offsetof(NF_TCP_CONN_INFO, remoteAddress) == 39, "remote address offset");
static_assert(sizeof(NF_EventHandler) == 16 * sizeof(void*), "C callback table");
static_assert(offsetof(NF_EventHandler, udpSend) == 13 * sizeof(void*), "UDP callback offset");
