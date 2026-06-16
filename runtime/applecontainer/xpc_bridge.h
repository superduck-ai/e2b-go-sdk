#ifndef E2B_APPLE_CONTAINER_XPC_BRIDGE_H
#define E2B_APPLE_CONTAINER_XPC_BRIDGE_H

#include <stdint.h>
#include <stddef.h>

#define XPC_ROUTE_KEY "com.apple.container.xpc.route"
#define XPC_ERROR_KEY "com.apple.container.xpc.error"

typedef void* xpc_bridge_conn_t;
typedef void* xpc_bridge_msg_t;

xpc_bridge_conn_t xpc_bridge_connect(const char* service_name);
void xpc_bridge_disconnect(xpc_bridge_conn_t conn);

xpc_bridge_msg_t xpc_bridge_msg_create(const char* route);
void xpc_bridge_msg_release(xpc_bridge_msg_t msg);

void xpc_bridge_msg_set_string(xpc_bridge_msg_t msg, const char* key, const char* value);
void xpc_bridge_msg_set_bool(xpc_bridge_msg_t msg, const char* key, int value);
void xpc_bridge_msg_set_uint64(xpc_bridge_msg_t msg, const char* key, uint64_t value);
void xpc_bridge_msg_set_data(xpc_bridge_msg_t msg, const char* key, const uint8_t* data, size_t len);
void xpc_bridge_msg_set_fd(xpc_bridge_msg_t msg, const char* key, int fd);

int xpc_bridge_send(xpc_bridge_conn_t conn, xpc_bridge_msg_t msg, uint64_t timeout_ms,
                    xpc_bridge_msg_t* reply, char* err_buf, size_t err_buf_len);
int xpc_bridge_reply_check_error(xpc_bridge_msg_t reply, char* err_buf, size_t err_buf_len);
const uint8_t* xpc_bridge_reply_get_data(xpc_bridge_msg_t reply, const char* key, size_t* len_out);

#endif
