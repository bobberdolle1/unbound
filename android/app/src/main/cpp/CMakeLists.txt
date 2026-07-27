cmake_minimum_required(VERSION 3.22.1)

project("unbound_tunnel")

add_library(
    unbound_tunnel
    SHARED
    unbound_tunnel.c
)

find_library(
    log-lib
    log
)

target_link_libraries(
    unbound_tunnel
    ${log-lib}
)
