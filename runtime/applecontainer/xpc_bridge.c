//go:build darwin && cgo

#include "xpc_bridge.h"

#include <dispatch/dispatch.h>
#include <errno.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <xpc/xpc.h>

#define XPC_BRIDGE_DEFAULT_TIMEOUT_MS 60000
#define XPC_BRIDGE_NSEC_PER_SEC 1000000000L
#define XPC_BRIDGE_NSEC_PER_MSEC 1000000L

typedef struct reply_state {
    pthread_mutex_t mutex;
    pthread_cond_t cond;
    int completed;
    int timed_out;
    int refs;
    xpc_object_t reply;
} reply_state_t;

static void set_error(char* err_buf, size_t err_buf_len, const char* message) {
    if (err_buf == NULL || err_buf_len == 0) {
        return;
    }
    if (message == NULL) {
        message = "unknown xpc bridge error";
    }
    snprintf(err_buf, err_buf_len, "%s", message);
}

static reply_state_t* reply_state_create(void) {
    reply_state_t* state = calloc(1, sizeof(reply_state_t));
    if (state == NULL) {
        return NULL;
    }
    if (pthread_mutex_init(&state->mutex, NULL) != 0) {
        free(state);
        return NULL;
    }
    if (pthread_cond_init(&state->cond, NULL) != 0) {
        pthread_mutex_destroy(&state->mutex);
        free(state);
        return NULL;
    }
    state->refs = 2;
    return state;
}

static void reply_state_release(reply_state_t* state) {
    if (state == NULL) {
        return;
    }

    int should_free = 0;
    pthread_mutex_lock(&state->mutex);
    state->refs -= 1;
    should_free = state->refs == 0;
    pthread_mutex_unlock(&state->mutex);

    if (should_free) {
        if (state->reply != NULL) {
            xpc_release(state->reply);
        }
        pthread_cond_destroy(&state->cond);
        pthread_mutex_destroy(&state->mutex);
        free(state);
    }
}

static int deadline_from_now(uint64_t timeout_ms, struct timespec* deadline) {
    if (deadline == NULL) {
        return -1;
    }
    if (clock_gettime(CLOCK_REALTIME, deadline) != 0) {
        return -1;
    }
    deadline->tv_sec += (time_t)(timeout_ms / 1000);
    deadline->tv_nsec += (long)((timeout_ms % 1000) * XPC_BRIDGE_NSEC_PER_MSEC);
    if (deadline->tv_nsec >= XPC_BRIDGE_NSEC_PER_SEC) {
        deadline->tv_sec += 1;
        deadline->tv_nsec -= XPC_BRIDGE_NSEC_PER_SEC;
    }
    return 0;
}

xpc_bridge_conn_t xpc_bridge_connect(const char* service_name) {
    if (service_name == NULL || service_name[0] == '\0') {
        return NULL;
    }

    xpc_connection_t conn = xpc_connection_create_mach_service(service_name, NULL, 0);
    if (conn == NULL) {
        return NULL;
    }
    xpc_connection_set_event_handler(conn, ^(xpc_object_t event) {
        (void)event;
    });
    xpc_connection_resume(conn);
    return (xpc_bridge_conn_t)conn;
}

void xpc_bridge_disconnect(xpc_bridge_conn_t conn) {
    if (conn == NULL) {
        return;
    }
    xpc_connection_cancel((xpc_connection_t)conn);
    xpc_release((xpc_object_t)conn);
}

xpc_bridge_msg_t xpc_bridge_msg_create(const char* route) {
    if (route == NULL || route[0] == '\0') {
        return NULL;
    }
    xpc_object_t msg = xpc_dictionary_create_empty();
    if (msg == NULL) {
        return NULL;
    }
    xpc_dictionary_set_string(msg, XPC_ROUTE_KEY, route);
    return (xpc_bridge_msg_t)msg;
}

void xpc_bridge_msg_release(xpc_bridge_msg_t msg) {
    if (msg != NULL) {
        xpc_release((xpc_object_t)msg);
    }
}

void xpc_bridge_msg_set_string(xpc_bridge_msg_t msg, const char* key, const char* value) {
    if (msg == NULL || key == NULL || value == NULL) {
        return;
    }
    xpc_dictionary_set_string((xpc_object_t)msg, key, value);
}

void xpc_bridge_msg_set_bool(xpc_bridge_msg_t msg, const char* key, int value) {
    if (msg == NULL || key == NULL) {
        return;
    }
    xpc_dictionary_set_bool((xpc_object_t)msg, key, value != 0);
}

void xpc_bridge_msg_set_uint64(xpc_bridge_msg_t msg, const char* key, uint64_t value) {
    if (msg == NULL || key == NULL) {
        return;
    }
    xpc_dictionary_set_uint64((xpc_object_t)msg, key, value);
}

void xpc_bridge_msg_set_data(xpc_bridge_msg_t msg, const char* key, const uint8_t* data, size_t len) {
    if (msg == NULL || key == NULL) {
        return;
    }
    if (data == NULL && len > 0) {
        return;
    }
    xpc_dictionary_set_data((xpc_object_t)msg, key, data, len);
}

void xpc_bridge_msg_set_fd(xpc_bridge_msg_t msg, const char* key, int fd) {
    if (msg == NULL || key == NULL || fd < 0) {
        return;
    }
    xpc_object_t xfd = xpc_fd_create(fd);
    if (xfd == NULL) {
        return;
    }
    xpc_dictionary_set_value((xpc_object_t)msg, key, xfd);
    xpc_release(xfd);
}

int xpc_bridge_send(xpc_bridge_conn_t conn, xpc_bridge_msg_t msg, uint64_t timeout_ms,
                    xpc_bridge_msg_t* reply, char* err_buf, size_t err_buf_len) {
    if (reply != NULL) {
        *reply = NULL;
    }
    if (conn == NULL) {
        set_error(err_buf, err_buf_len, "xpc connection is nil");
        return -1;
    }
    if (msg == NULL) {
        set_error(err_buf, err_buf_len, "xpc message is nil");
        return -1;
    }
    if (timeout_ms == 0) {
        timeout_ms = XPC_BRIDGE_DEFAULT_TIMEOUT_MS;
    }

    reply_state_t* state = reply_state_create();
    if (state == NULL) {
        set_error(err_buf, err_buf_len, "allocate xpc reply state");
        return -1;
    }

    xpc_connection_send_message_with_reply((xpc_connection_t)conn, (xpc_object_t)msg,
                                           dispatch_get_global_queue(QOS_CLASS_DEFAULT, 0),
                                           ^(xpc_object_t event) {
        xpc_object_t retained = NULL;
        if (event != NULL) {
            retained = xpc_retain(event);
        }

        pthread_mutex_lock(&state->mutex);
        if (state->timed_out) {
            pthread_mutex_unlock(&state->mutex);
            if (retained != NULL) {
                xpc_release(retained);
            }
            reply_state_release(state);
            return;
        }

        state->reply = retained;
        state->completed = 1;
        pthread_cond_signal(&state->cond);
        pthread_mutex_unlock(&state->mutex);
        reply_state_release(state);
    });

    struct timespec deadline;
    if (deadline_from_now(timeout_ms, &deadline) != 0) {
        set_error(err_buf, err_buf_len, "compute xpc request deadline");
        pthread_mutex_lock(&state->mutex);
        state->timed_out = 1;
        pthread_mutex_unlock(&state->mutex);
        reply_state_release(state);
        return -1;
    }

    int timed_out = 0;
    int wait_error = 0;
    pthread_mutex_lock(&state->mutex);
    while (!state->completed) {
        int wait_rc = pthread_cond_timedwait(&state->cond, &state->mutex, &deadline);
        if (wait_rc == ETIMEDOUT) {
            state->timed_out = 1;
            timed_out = 1;
            break;
        }
        if (wait_rc != 0) {
            state->timed_out = 1;
            wait_error = wait_rc;
            break;
        }
    }

    if (timed_out || wait_error != 0) {
        pthread_mutex_unlock(&state->mutex);
        set_error(err_buf, err_buf_len, timed_out ? "xpc request timed out" : "wait for xpc reply failed");
        reply_state_release(state);
        return -1;
    }

    if (reply != NULL) {
        *reply = (xpc_bridge_msg_t)state->reply;
        state->reply = NULL;
    }
    pthread_mutex_unlock(&state->mutex);
    reply_state_release(state);
    return 0;
}

int xpc_bridge_reply_check_error(xpc_bridge_msg_t reply, char* err_buf, size_t err_buf_len) {
    if (reply == NULL) {
        set_error(err_buf, err_buf_len, "xpc reply is nil");
        return -1;
    }

    xpc_object_t object = (xpc_object_t)reply;
    xpc_type_t type = xpc_get_type(object);
    if (type == XPC_TYPE_ERROR) {
        const char* desc = xpc_dictionary_get_string(object, XPC_ERROR_KEY_DESCRIPTION);
        set_error(err_buf, err_buf_len, desc != NULL ? desc : "xpc transport error");
        return -1;
    }
    if (type != XPC_TYPE_DICTIONARY) {
        char* desc = xpc_copy_description(object);
        set_error(err_buf, err_buf_len, desc != NULL ? desc : "unexpected xpc reply type");
        free(desc);
        return -1;
    }

    size_t error_len = 0;
    const void* error_data = xpc_dictionary_get_data(object, XPC_ERROR_KEY, &error_len);
    if (error_data != NULL && error_len > 0) {
        if (err_buf != NULL && err_buf_len > 0) {
            size_t copy_len = error_len < err_buf_len - 1 ? error_len : err_buf_len - 1;
            memcpy(err_buf, error_data, copy_len);
            err_buf[copy_len] = '\0';
        }
        return -1;
    }

    return 0;
}

const uint8_t* xpc_bridge_reply_get_data(xpc_bridge_msg_t reply, const char* key, size_t* len_out) {
    if (len_out != NULL) {
        *len_out = 0;
    }
    if (reply == NULL || key == NULL) {
        return NULL;
    }
    if (xpc_get_type((xpc_object_t)reply) != XPC_TYPE_DICTIONARY) {
        return NULL;
    }
    size_t len = 0;
    const void* data = xpc_dictionary_get_data((xpc_object_t)reply, key, &len);
    if (len_out != NULL) {
        *len_out = len;
    }
    return (const uint8_t*)data;
}
