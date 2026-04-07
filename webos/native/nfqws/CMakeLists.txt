cmake_minimum_required(VERSION 3.10)
project(nfqws-webos C)

# ============================================================================
# Unbound WebOS — CMake build configuration for nfqws
# Cross-compiles the zapret nfqws engine for LG WebOS ARM
# ============================================================================

set(CMAKE_C_STANDARD 99)
set(CMAKE_C_STANDARD_REQUIRED ON)

# Optimization flags
set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -Os -flto=auto -ffunction-sections -fdata-sections")
set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} -flto=auto -Wl,--gc-sections")

# Source files from zapret nfq directory
set(NFQWS_SRC_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../zapret/nfq")

# Collect all C source files
file(GLOB NFQWS_SOURCES 
  "${NFQWS_SRC_DIR}/*.c"
  "${NFQWS_SRC_DIR}/crypto/*.c"
)

# Include directories
include_directories(
  ${NFQWS_SRC_DIR}
  ${NFQWS_SRC_DIR}/crypto
)

# Link libraries (must be cross-compiled for WebOS ARM first)
set(NFQWS_LIBS
  z
  netfilter_queue
  nfnetlink
  mnl
)

# Build the binary
add_executable(nfqws ${NFQWS_SOURCES})
target_link_libraries(nfqws ${NFQWS_LIBS})

# Strip the binary in release mode
if(CMAKE_BUILD_TYPE STREQUAL "Release")
  add_custom_command(TARGET nfqws POST_BUILD
    COMMAND ${CMAKE_STRIP} $<TARGET_FILE:nfqws>
    COMMENT "Stripping nfqws binary"
  )
endif()

# Install to app bundle
install(TARGETS nfqws DESTINATION bin)
