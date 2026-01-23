/* src/include/pg_config.h.  Generated from src/include/pg_config.h.in by src/tools/msvc/Solution.pm.  */
/* src/include/pg_config.h.in.  Generated from configure.ac by autoheader.  */

/* Define to the type of arg 1 of 'accept' */
#define ACCEPT_TYPE_ARG1 unsigned int

/* Define to the type of arg 2 of 'accept' */
#define ACCEPT_TYPE_ARG2 struct sockaddr *

/* Define to the type of arg 3 of 'accept' */
#define ACCEPT_TYPE_ARG3 int

/* Define to the return type of 'accept' */
#define ACCEPT_TYPE_RETURN unsigned int PASCAL

/* Define if building universal (internal helper macro) */
/* #undef AC_APPLE_UNIVERSAL_BUILD */

/* The normal alignment of `double', in bytes. */
#define ALIGNOF_DOUBLE 8

/* The normal alignment of `int', in bytes. */
#define ALIGNOF_INT 4

/* The normal alignment of `long', in bytes. */
#define ALIGNOF_LONG 4

/* The normal alignment of `long long int', in bytes. */
#define ALIGNOF_LONG_LONG_INT 8

/* The normal alignment of `PG_INT128_TYPE', in bytes. */
/* #undef ALIGNOF_PG_INT128_TYPE */

/* The normal alignment of `short', in bytes. */
#define ALIGNOF_SHORT 2

/* Size of a disk block --- this also limits the size of a tuple. You can set
   it bigger if you need bigger tuples (although TOAST should reduce the need
   to have large tuples, since fields can be spread across multiple tuples).
   BLCKSZ must be a power of 2. The maximum possible value of BLCKSZ is
   currently 2^15 (32768). This is determined by the 15-bit widths of the
   lp_off and lp_len fields in ItemIdData (see include/storage/itemid.h).
   Changing BLCKSZ requires an initdb. */
#define BLCKSZ 8192

/* Saved arguments from configure */
#define CONFIGURE_ARGS "--enable-thread-safety --enable-nls --with-ldap --with-ssl=openssl --with-uuid --with-libxml --with-libxslt --with-lz4 --with-icu --with-tcl --with-perl --with-python"

/* Define to the default TCP port number on which the server listens and to
   which clients will try to connect. This can be overridden at run-time, but
   it's convenient if your clients have the right default compiled in.
   (--with-pgport=PORTNUM) */
#define DEF_PGPORT 5432

/* Define to the default TCP port number as a string constant. */
#define DEF_PGPORT_STR "5432"

/* Define to build with GSSAPI support. (--with-gssapi) */
/* #undef ENABLE_GSS */

/* Define to 1 if you want National Language Support. (--enable-nls) */
#define ENABLE_NLS 1

/* Define to 1 to build client libraries as thread-safe code.
   (--enable-thread-safety) */
#define ENABLE_THREAD_SAFETY 1

/* Define to 1 if gettimeofday() takes only 1 argument. */
/* #undef GETTIMEOFDAY_1ARG */

#ifdef GETTIMEOFDAY_1ARG
# define gettimeofday(a,b) gettimeofday(a)
#endif

/* Define to 1 if you have the `append_history' function. */
/* #undef HAVE_APPEND_HISTORY */

/* Define to 1 if you have the `ASN1_STRING_get0_data' function. */
#define HAVE_ASN1_STRING_GET0_DATA 1

/* Define to 1 if you want to use atomics if available. */
#define HAVE_ATOMICS 1

/* Define to 1 if you have the <atomic.h> header file. */
/* #undef HAVE_ATOMIC_H */

/* Define to 1 if you have the `backtrace_symbols' function. */
/* #undef HAVE_BACKTRACE_SYMBOLS */

/* Define to 1 if you have the `BIO_meth_new' function. */
#define HAVE_BIO_METH_NEW 1

/* Define to 1 if you have the `clock_gettime' function. */
/* #undef HAVE_CLOCK_GETTIME */

/* Define to 1 if your compiler handles computed gotos. */
/* #undef HAVE_COMPUTED_GOTO */

/* Define to 1 if you have the `copyfile' function. */
/* #undef HAVE_COPYFILE */

/* Define to 1 if you have the <copyfile.h> header file. */
/* #undef HAVE_COPYFILE_H */

/* Define to 1 if you have the <crtdefs.h> header file. */
/* #undef HAVE_CRTDEFS_H */

/* Define to 1 if you have the `CRYPTO_lock' function. */
/* #undef HAVE_CRYPTO_LOCK */

/* Define to 1 if you have the declaration of `fdatasync', and to 0 if you
   don't. */
#define HAVE_DECL_FDATASYNC 0

/* Define to 1 if you have the declaration of `F_FULLFSYNC', and to 0 if you
   don't. */
#define HAVE_DECL_F_FULLFSYNC 0

/* Define to 1 if you have the declaration of
   `LLVMCreateGDBRegistrationListener', and to 0 if you don't. */
#define HAVE_DECL_LLVMCREATEGDBREGISTRATIONLISTENER 0

/* Define to 1 if you have the declaration of
   `LLVMCreatePerfJITEventListener', and to 0 if you don't. */
#define HAVE_DECL_LLVMCREATEPERFJITEVENTLISTENER 0

/* Define to 1 if you have the declaration of `LLVMGetHostCPUFeatures', and to
   0 if you don't. */
#define HAVE_DECL_LLVMGETHOSTCPUFEATURES 0

/* Define to 1 if you have the declaration of `LLVMGetHostCPUName', and to 0
   if you don't. */
#define HAVE_DECL_LLVMGETHOSTCPUNAME 0

/* Define to 1 if you have the declaration of `LLVMOrcGetSymbolAddressIn', and
   to 0 if you don't. */
#define HAVE_DECL_LLVMORCGETSYMBOLADDRESSIN 0

/* Define to 1 if you have the declaration of `posix_fadvise', and to 0 if you
   don't. */
#define HAVE_DECL_POSIX_FADVISE 0

/* Define to 1 if you have the declaration of `preadv', and to 0 if you don't.
   */
#define HAVE_DECL_PREADV 0

/* Define to 1 if you have the declaration of `pwritev', and to 0 if you
   don't. */
#define HAVE_DECL_PWRITEV 0

/* Define to 1 if you have the declaration of `RTLD_GLOBAL', and to 0 if you
   don't. */
#define HAVE_DECL_RTLD_GLOBAL 0

/* Define to 1 if you have the declaration of `RTLD_NOW', and to 0 if you
   don't. */
#define HAVE_DECL_RTLD_NOW 0

/* Define to 1 if you have the declaration of `strlcat', and to 0 if you
   don't. */
#define HAVE_DECL_STRLCAT 0

/* Define to 1 if you have the declaration of `strlcpy', and to 0 if you
   don't. */
#define HAVE_DECL_STRLCPY 0

/* Define to 1 if you have the declaration of `strnlen', and to 0 if you
   don't. */
#define HAVE_DECL_STRNLEN 1

/* Define to 1 if you have the declaration of `strtoll', and to 0 if you
   don't. */
#define HAVE_DECL_STRTOLL 1

/* Define to 1 if you have the declaration of `strtoull', and to 0 if you
   don't. */
#define HAVE_DECL_STRTOULL 1

/* Define to 1 if you have the `dlopen' function. */
/* #undef HAVE_DLOPEN */

/* Define to 1 if you have the <editline/history.h> header file. */
/* #undef HAVE_EDITLINE_HISTORY_H */

/* Define to 1 if you have the <editline/readline.h> header file. */
/* #undef HAVE_EDITLINE_READLINE_H */

/* Define to 1 if you have the <execinfo.h> header file. */
/* #undef HAVE_EXECINFO_H */

/* Define to 1 if you have the `explicit_bzero' function. */
/* #undef HAVE_EXPLICIT_BZERO */

/* Define to 1 if you have the `fdatasync' function. */
/* #undef HAVE_FDATASYNC */

/* Define to 1 if you have the `fls' function. */
/* #undef HAVE_FLS */

/* Define to 1 if fseeko (and presumably ftello) exists and is declared. */
#define HAVE_FSEEKO 1

/* Define to 1 if your compiler understands __func__. */
/* #undef HAVE_FUNCNAME__FUNC */

/* Define to 1 if your compiler understands __FUNCTION__. */
#define HAVE_FUNCNAME__FUNCTION 1

/* Define to 1 if you have __atomic_compare_exchange_n(int *, int *, int). */
/* #undef HAVE_GCC__ATOMIC_INT32_CAS */

/* Define to 1 if you have __atomic_compare_exchange_n(int64 *, int64 *,
   int64). */
/* #undef HAVE_GCC__ATOMIC_INT64_CAS */

/* Define to 1 if you have __sync_lock_test_and_set(char *) and friends. */
/* #undef HAVE_GCC__SYNC_CHAR_TAS */

/* Define to 1 if you have __sync_val_compare_and_swap(int *, int, int). */
/* #undef HAVE_GCC__SYNC_INT32_CAS */

/* Define to 1 if you have __sync_lock_test_and_set(int *) and friends. */
/* #undef HAVE_GCC__SYNC_INT32_TAS */

/* Define to 1 if you have __sync_val_compare_and_swap(int64 *, int64, int64).
   */
/* #undef HAVE_GCC__SYNC_INT64_CAS */

/* Define to 1 if you have the `getaddrinfo' function. */
/* #undef HAVE_GETADDRINFO */

/* Define to 1 if you have the `gethostbyname_r' function. */
/* #undef HAVE_GETHOSTBYNAME_R */

/* Define to 1 if you have the `getifaddrs' function. */
/* #undef HAVE_GETIFADDRS */

/* Define to 1 if you have the `getopt' function. */
/* #undef HAVE_GETOPT */

/* Define to 1 if you have the <getopt.h> header file. */
/* #undef HAVE_GETOPT_H */

/* Define to 1 if you have the `getopt_long' function. */
/* #undef HAVE_GETOPT_LONG */

/* Define to 1 if you have the `getpeereid' function. */
/* #undef HAVE_GETPEEREID */

/* Define to 1 if you have the `getpeerucred' function. */
/* #undef HAVE_GETPEERUCRED */

/* Define to 1 if you have the `getpwuid_r' function. */
/* #undef HAVE_GETPWUID_R */

/* Define to 1 if you have the `getrlimit' function. */
/* #undef HAVE_GETRLIMIT */

/* Define to 1 if you have the `getrusage' function. */
/* #undef HAVE_GETRUSAGE */

/* Define to 1 if you have the `gettimeofday' function. */
/* #undef HAVE_GETTIMEOFDAY */

/* Define to 1 if you have the <gssapi/gssapi.h> header file. */
/* #undef HAVE_GSSAPI_GSSAPI_H */

/* Define to 1 if you have the <gssapi.h> header file. */
/* #undef HAVE_GSSAPI_H */

/* Define to 1 if you have the <history.h> header file. */
/* #undef HAVE_HISTORY_H */

/* Define to 1 if you have the `history_truncate_file' function. */
/* #undef HAVE_HISTORY_TRUNCATE_FILE */

/* Define to 1 if you have the `HMAC_CTX_free' function. */
#define HAVE_HMAC_CTX_FREE 1

/* Define to 1 if you have the `HMAC_CTX_new' function. */
#define HAVE_HMAC_CTX_NEW 1

/* Define to 1 if you have the <ifaddrs.h> header file. */
/* #undef HAVE_IFADDRS_H */

/* Define to 1 if you have the `inet_aton' function. */
/* #undef HAVE_INET_ATON */

/* Define to 1 if the system has the type `int64'. */
/* #undef HAVE_INT64 */

/* Define to 1 if the system has the type `int8'. */
/* #undef HAVE_INT8 */

/* Define to 1 if you have the <inttypes.h> header file. */
/* #undef HAVE_INTTYPES_H */

/* Define to 1 if you have the global variable 'int opterr'. */
/* #undef HAVE_INT_OPTERR */

/* Define to 1 if you have the global variable 'int optreset'. */
/* #undef HAVE_INT_OPTRESET */

/* Define to 1 if you have the global variable 'int timezone'. */
#define HAVE_INT_TIMEZONE 1

/* Define to 1 if you have support for IPv6. */
#define HAVE_IPV6 1

/* Define to 1 if __builtin_constant_p(x) implies "i"(x) acceptance. */
/* #undef HAVE_I_CONSTRAINT__BUILTIN_CONSTANT_P */

/* Define to 1 if you have the `kqueue' function. */
/* #undef HAVE_KQUEUE */

/* Define to 1 if you have the <langinfo.h> header file. */
/* #undef HAVE_LANGINFO_H */

/* Define to 1 if you have the <ldap.h> header file. */
/* #undef HAVE_LDAP_H */

/* Define to 1 if you have the `ldap_initialize' function. */
/* #undef HAVE_LDAP_INITIALIZE */

/* Define to 1 if you have the `crypto' library (-lcrypto). */
/* #undef HAVE_LIBCRYPTO */

/* Define to 1 if you have the `ldap' library (-lldap). */
/* #undef HAVE_LIBLDAP */

/* Define to 1 if you have the `lz4' library (-llz4). */
#define HAVE_LIBLZ4 1

/* Define to 1 if you have the `m' library (-lm). */
/* #undef HAVE_LIBM */

/* Define to 1 if you have the `pam' library (-lpam). */
/* #undef HAVE_LIBPAM */

/* Define if you have a function readline library */
/* #undef HAVE_LIBREADLINE */

/* Define to 1 if you have the `selinux' library (-lselinux). */
/* #undef HAVE_LIBSELINUX */

/* Define to 1 if you have the `ssl' library (-lssl). */
/* #undef HAVE_LIBSSL */

/* Define to 1 if you have the `wldap32' library (-lwldap32). */
/* #undef HAVE_LIBWLDAP32 */

/* Define to 1 if you have the `xml2' library (-lxml2). */
#define HAVE_LIBXML2 1

/* Define to 1 if you have the `xslt' library (-lxslt). */
#define HAVE_LIBXSLT 1

/* Define to 1 if you have the `z' library (-lz). */
#define HAVE_LIBZ 1

/* Define to 1 if you have the `link' function. */
/* #undef HAVE_LINK */

/* Define to 1 if the system has the type `locale_t'. */
#define HAVE_LOCALE_T 1

/* Define to 1 if `long int' works and is 64 bits. */
/* #undef HAVE_LONG_INT_64 */

/* Define to 1 if `long long int' works and is 64 bits. */
#define HAVE_LONG_LONG_INT_64 1

/* Define to 1 if you have the <lz4.h> header file. */
#define HAVE_LZ4_H 1

/* Define to 1 if you have the <mbarrier.h> header file. */
/* #undef HAVE_MBARRIER_H */

/* Define to 1 if you have the `mbstowcs_l' function. */
#define HAVE_MBSTOWCS_L 1

/* Define to 1 if you have the <memory.h> header file. */
#define HAVE_MEMORY_H 1

/* Define to 1 if you have the `memset_s' function. */
/* #undef HAVE_MEMSET_S */

/* Define to 1 if the system has the type `MINIDUMP_TYPE'. */
#define HAVE_MINIDUMP_TYPE 1

/* Define to 1 if you have the `mkdtemp' function. */
/* #undef HAVE_MKDTEMP */

/* Define to 1 if you have the <netinet/tcp.h> header file. */
/* #undef HAVE_NETINET_TCP_H */

/* Define to 1 if you have the <net/if.h> header file. */
/* #undef HAVE_NET_IF_H */

/* Define to 1 if you have the `OPENSSL_init_ssl' function. */
#define HAVE_OPENSSL_INIT_SSL 1

/* Define to 1 if you have the <ossp/uuid.h> header file. */
/* #undef HAVE_OSSP_UUID_H */

/* Define to 1 if you have the <pam/pam_appl.h> header file. */
/* #undef HAVE_PAM_PAM_APPL_H */

/* Define to 1 if you have the `poll' function. */
/* #undef HAVE_POLL */

/* Define to 1 if you have the <poll.h> header file. */
/* #undef HAVE_POLL_H */

/* Define to 1 if you have the `posix_fadvise' function. */
/* #undef HAVE_POSIX_FADVISE */

/* Define to 1 if you have the `posix_fallocate' function. */
/* #undef HAVE_POSIX_FALLOCATE */

/* Define to 1 if the assembler supports PPC's LWARX mutex hint bit. */
/* #undef HAVE_PPC_LWARX_MUTEX_HINT */

/* Define to 1 if you have the `ppoll' function. */
/* #undef HAVE_PPOLL */

/* Define to 1 if you have the `pread' function. */
/* #undef HAVE_PREAD */

/* Define to 1 if you have the `pstat' function. */
/* #undef HAVE_PSTAT */

/* Define to 1 if the PS_STRINGS thing exists. */
/* #undef HAVE_PS_STRINGS */

/* Define if you have POSIX threads libraries and header files. */
/* #undef HAVE_PTHREAD */

/* Define to 1 if you have the `pthread_barrier_wait' function. */
/* #undef HAVE_PTHREAD_BARRIER_WAIT */

/* Define to 1 if you have the `pthread_is_threaded_np' function. */
/* #undef HAVE_PTHREAD_IS_THREADED_NP */

/* Have PTHREAD_PRIO_INHERIT. */
/* #undef HAVE_PTHREAD_PRIO_INHERIT */

/* Define to 1 if you have the `pwrite' function. */
/* #undef HAVE_PWRITE */

/* Define to 1 if you have the `random' function. */
/* #undef HAVE_RANDOM */

/* Define to 1 if you have the <readline.h> header file. */
/* #undef HAVE_READLINE_H */

/* Define to 1 if you have the <readline/history.h> header file. */
/* #undef HAVE_READLINE_HISTORY_H */

/* Define to 1 if you have the <readline/readline.h> header file. */
/* #undef HAVE_READLINE_READLINE_H */

/* Define to 1 if you have the `readlink' function. */
/* #undef HAVE_READLINK */

/* Define to 1 if you have the `readv' function. */
/* #undef HAVE_READV */

/* Define to 1 if you have the global variable
   'rl_completion_append_character'. */
/* #undef HAVE_RL_COMPLETION_APPEND_CHARACTER */

/* Define to 1 if you have the `rl_completion_matches' function. */
/* #undef HAVE_RL_COMPLETION_MATCHES */

/* Define to 1 if you have the global variable 'rl_completion_suppress_quote'.
   */
/* #undef HAVE_RL_COMPLETION_SUPPRESS_QUOTE */

/* Define to 1 if you have the `rl_filename_completion_function' function. */
/* #undef HAVE_RL_FILENAME_COMPLETION_FUNCTION */

/* Define to 1 if you have the global variable 'rl_filename_quote_characters'.
   */
/* #undef HAVE_RL_FILENAME_QUOTE_CHARACTERS */

/* Define to 1 if you have the global variable 'rl_filename_quoting_function'.
   */
/* #undef HAVE_RL_FILENAME_QUOTING_FUNCTION */

/* Define to 1 if you have the `rl_reset_screen_size' function. */
/* #undef HAVE_RL_RESET_SCREEN_SIZE */

/* Define to 1 if you have the <security/pam_appl.h> header file. */
/* #undef HAVE_SECURITY_PAM_APPL_H */

/* Define to 1 if you have the `setenv' function. */
/* #undef HAVE_SETENV */

/* Define to 1 if you have the `setproctitle' function. */
/* #undef HAVE_SETPROCTITLE */

/* Define to 1 if you have the `setproctitle_fast' function. */
/* #undef HAVE_SETPROCTITLE_FAST */

/* Define to 1 if you have the `setsid' function. */
/* #undef HAVE_SETSID */

/* Define to 1 if you have the `shm_open' function. */
/* #undef HAVE_SHM_OPEN */

/* Define to 1 if you have spinlocks. */
#define HAVE_SPINLOCKS 1

/* Define to 1 if you have the `srandom' function. */
/* #undef HAVE_SRANDOM */

/* Define to 1 if stdbool.h conforms to C99. */
#define HAVE_STDBOOL_H 1

/* Define to 1 if you have the <stdint.h> header file. */
#define HAVE_STDINT_H 1

/* Define to 1 if you have the <stdlib.h> header file. */
#define HAVE_STDLIB_H 1

/* Define to 1 if you have the `strchrnul' function. */
/* #undef HAVE_STRCHRNUL */

/* Define to 1 if you have the `strerror_r' function. */
/* #undef HAVE_STRERROR_R */

/* Define to 1 if you have the <strings.h> header file. */
/* #undef HAVE_STRINGS_H */

/* Define to 1 if you have the <string.h> header file. */
#define HAVE_STRING_H 1

/* Define to 1 if you have the `strlcat' function. */
/* #undef HAVE_STRLCAT */

/* Define to 1 if you have the `strlcpy' function. */
/* #undef HAVE_STRLCPY */

/* Define to 1 if you have the `strnlen' function. */
#define HAVE_STRNLEN 1

/* Define to 1 if you have the `strsignal' function. */
/* #undef HAVE_STRSIGNAL */

/* Define to 1 if you have the `strtof' function. */
#define HAVE_STRTOF 1

/* Define to 1 if you have the `strtoll' function. */
#define HAVE_STRTOLL 1

/* Define to 1 if you have the `strtoq' function. */
/* #undef HAVE_STRTOQ */

/* Define to 1 if you have the `strtoull' function. */
#define HAVE_STRTOULL 1

/* Define to 1 if you have the `strtouq' function. */
/* #undef HAVE_STRTOUQ */

/* Define to 1 if the system has the type `struct addrinfo'. */
#define HAVE_STRUCT_ADDRINFO 1

/* Define to 1 if the system has the type `struct cmsgcred'. */
/* #undef HAVE_STRUCT_CMSGCRED */

/* Define to 1 if the system has the type `struct option'. */
/* #undef HAVE_STRUCT_OPTION */

/* Define to 1 if `sa_len' is a member of `struct sockaddr'. */
/* #undef HAVE_STRUCT_SOCKADDR_SA_LEN */

/* Define to 1 if the system has the type `struct sockaddr_storage'. */
#define HAVE_STRUCT_SOCKADDR_STORAGE 1

/* Define to 1 if `ss_family' is a member of `struct sockaddr_storage'. */
#define HAVE_STRUCT_SOCKADDR_STORAGE_SS_FAMILY 1

/* Define to 1 if `ss_len' is a member of `struct sockaddr_storage'. */
/* #undef HAVE_STRUCT_SOCKADDR_STORAGE_SS_LEN */

/* Define to 1 if `__ss_family' is a member of `struct sockaddr_storage'. */
/* #undef HAVE_STRUCT_SOCKADDR_STORAGE___SS_FAMILY */

/* Define to 1 if `__ss_len' is a member of `struct sockaddr_storage'. */
/* #undef HAVE_STRUCT_SOCKADDR_STORAGE___SS_LEN */

/* Define to 1 if the system has the type `struct sockaddr_un'. */
/* #undef HAVE_STRUCT_SOCKADDR_UN */

/* Define to 1 if `tm_zone' is a member of `struct tm'. */
/* #undef HAVE_STRUCT_TM_TM_ZONE */

/* Define to 1 if you have the `symlink' function. */
#define HAVE_SYMLINK 1

/* Define to 1 if you have the `syncfs' function. */
/* #undef HAVE_SYNCFS */

/* Define to 1 if you have the `sync_file_range' function. */
/* #undef HAVE_SYNC_FILE_RANGE */

/* Define to 1 if you have the syslog interface. */
/* #undef HAVE_SYSLOG */

/* Define to 1 if you have the <sys/epoll.h> header file. */
/* #undef HAVE_SYS_EPOLL_H */

/* Define to 1 if you have the <sys/event.h> header file. */
/* #undef HAVE_SYS_EVENT_H */

/* Define to 1 if you have the <sys/ipc.h> header file. */
/* #undef HAVE_SYS_IPC_H */

/* Define to 1 if you have the <sys/personality.h> header file. */
/* #undef HAVE_SYS_PERSONALITY_H */

/* Define to 1 if you have the <sys/prctl.h> header file. */
/* #undef HAVE_SYS_PRCTL_H */

/* Define to 1 if you have the <sys/procctl.h> header file. */
/* #undef HAVE_SYS_PROCCTL_H */

/* Define to 1 if you have the <sys/pstat.h> header file. */
/* #undef HAVE_SYS_PSTAT_H */

/* Define to 1 if you have the <sys/resource.h> header file. */
/* #undef HAVE_SYS_RESOURCE_H */

/* Define to 1 if you have the <sys/select.h> header file. */
/* #undef HAVE_SYS_SELECT_H */

/* Define to 1 if you have the <sys/sem.h> header file. */
/* #undef HAVE_SYS_SEM_H */

/* Define to 1 if you have the <sys/shm.h> header file. */
/* #undef HAVE_SYS_SHM_H */

/* Define to 1 if you have the <sys/signalfd.h> header file. */
/* #undef HAVE_SYS_SIGNALFD_H */

/* Define to 1 if you have the <sys/sockio.h> header file. */
/* #undef HAVE_SYS_SOCKIO_H */

/* Define to 1 if you have the <sys/stat.h> header file. */
#define HAVE_SYS_STAT_H 1

/* Define to 1 if you have the <sys/tas.h> header file. */
/* #undef HAVE_SYS_TAS_H */

/* Define to 1 if you have the <sys/types.h> header file. */
#define HAVE_SYS_TYPES_H 1

/* Define to 1 if you have the <sys/ucred.h> header file. */
/* #undef HAVE_SYS_UCRED_H */

/* Define to 1 if you have the <sys/uio.h> header file. */
/* #undef HAVE_SYS_UIO_H */

/* Define to 1 if you have the <sys/un.h> header file. */
/* #undef HAVE_SYS_UN_H */

/* Define to 1 if you have the <termios.h> header file. */
/* #undef HAVE_TERMIOS_H */

/* Define to 1 if your compiler understands `typeof' or something similar. */
/* #undef HAVE_TYPEOF */

/* Define to 1 if you have the <ucred.h> header file. */
/* #undef HAVE_UCRED_H */

/* Define to 1 if the system has the type `uint64'. */
/* #undef HAVE_UINT64 */

/* Define to 1 if the system has the type `uint8'. */
/* #undef HAVE_UINT8 */

/* Define to 1 if the system has the type `union semun'. */
/* #undef HAVE_UNION_SEMUN */

/* Define to 1 if you have the <unistd.h> header file. */
#define HAVE_UNISTD_H 1

/* Define to 1 if you have the `unsetenv' function. */
/* #undef HAVE_UNSETENV */

/* Define to 1 if you have the `uselocale' function. */
/* #undef HAVE_USELOCALE */

/* Define to 1 if you have BSD UUID support. */
/* #undef HAVE_UUID_BSD */

/* Define to 1 if you have E2FS UUID support. */
/* #undef HAVE_UUID_E2FS */

/* Define to 1 if you have the <uuid.h> header file. */
#define HAVE_UUID_H 1

/* Define to 1 if you have OSSP UUID support. */
#define HAVE_UUID_OSSP 1

/* Define to 1 if you have the <uuid/uuid.h> header file. */
/* #undef HAVE_UUID_UUID_H */

/* Define to 1 if you have the `wcstombs_l' function. */
#define HAVE_WCSTOMBS_L 1

/* Define to 1 if you have the <wctype.h> header file. */
#define HAVE_WCTYPE_H 1

/* Define to 1 if you have the <winldap.h> header file. */
/* #undef HAVE_WINLDAP_H */

/* Define to 1 if you have the `writev' function. */
/* #undef HAVE_WRITEV */

/* Define to 1 if you have the `X509_get_signature_info' function. */
#define HAVE_X509_GET_SIGNATURE_INFO 1

/* Define to 1 if you have the `X509_get_signature_nid' function. */
#define HAVE_X509_GET_SIGNATURE_NID 1

/* Define to 1 if the assembler supports X86_64's POPCNTQ instruction. */
/* #undef HAVE_X86_64_POPCNTQ */

/* Define to 1 if the system has the type `_Bool'. */
/* #undef HAVE__BOOL */

/* Define to 1 if your compiler understands __builtin_bswap16. */
/* #undef HAVE__BUILTIN_BSWAP16 */

/* Define to 1 if your compiler understands __builtin_bswap32. */
/* #undef HAVE__BUILTIN_BSWAP32 */

/* Define to 1 if your compiler understands __builtin_bswap64. */
/* #undef HAVE__BUILTIN_BSWAP64 */

/* Define to 1 if your compiler understands __builtin_clz. */
/* #undef HAVE__BUILTIN_CLZ */

/* Define to 1 if your compiler understands __builtin_constant_p. */
/* #undef HAVE__BUILTIN_CONSTANT_P */

/* Define to 1 if your compiler understands __builtin_ctz. */
/* #undef HAVE__BUILTIN_CTZ */

/* Define to 1 if your compiler understands __builtin_frame_address. */
/* #undef HAVE__BUILTIN_FRAME_ADDRESS */

/* Define to 1 if your compiler understands __builtin_$op_overflow. */
/* #undef HAVE__BUILTIN_OP_OVERFLOW */

/* Define to 1 if your compiler understands __builtin_popcount. */
/* #undef HAVE__BUILTIN_POPCOUNT */

/* Define to 1 if your compiler understands __builtin_types_compatible_p. */
/* #undef HAVE__BUILTIN_TYPES_COMPATIBLE_P */

/* Define to 1 if your compiler understands __builtin_unreachable. */
/* #undef HAVE__BUILTIN_UNREACHABLE */

/* Define to 1 if you have the `_configthreadlocale' function. */
#define HAVE__CONFIGTHREADLOCALE 1

/* Define to 1 if you have __cpuid. */
#define HAVE__CPUID 1

/* Define to 1 if you have __get_cpuid. */
/* #undef HAVE__GET_CPUID */

/* Define to 1 if your compiler understands _Static_assert. */
/* #undef HAVE__STATIC_ASSERT */

/* Define to 1 if you have the `__strtoll' function. */
/* #undef HAVE___STRTOLL */

/* Define to 1 if you have the `__strtoull' function. */
/* #undef HAVE___STRTOULL */

/* Define to the appropriate printf length modifier for 64-bit ints. */
#define INT64_MODIFIER "ll"

/* Define to 1 if `locale_t' requires <xlocale.h>. */
/* #undef LOCALE_T_IN_XLOCALE */

/* Define as the maximum alignment requirement of any C data type. */
#define MAXIMUM_ALIGNOF 8

/* Define bytes to use libc memset(). */
#define MEMSET_LOOP_LIMIT 1024

/* Define to the OpenSSL API version in use. This avoids deprecation warnings
   from newer OpenSSL versions. */
#define OPENSSL_API_COMPAT 0x10001000L

/* Define to the address where bug reports for this package should be sent. */
#define PACKAGE_BUGREPORT "pgsql-bugs@lists.postgresql.org"

/* Define to the full name of this package. */
#define PACKAGE_NAME "PostgreSQL"

/* Define to the full name and version of this package. */
#define PACKAGE_STRING "PostgreSQL 14.12"

/* Define to the one symbol short name of this package. */
#define PACKAGE_TARNAME "postgresql"

/* Define to the home page for this package. */
#define PACKAGE_URL "https://www.postgresql.org/"

/* Define to the version of this package. */
#define PACKAGE_VERSION "14.12"

/* Define to the name of a signed 128-bit integer type. */
/* #undef PG_INT128_TYPE */

/* Define to the name of a signed 64-bit integer type. */
#define PG_INT64_TYPE long long int

/* Define to the name of the default PostgreSQL service principal in Kerberos
   (GSSAPI). (--with-krb-srvnam=NAME) */
#define PG_KRB_SRVNAM "postgres"

/* PostgreSQL major version as a string */
#define PG_MAJORVERSION "14"

/* PostgreSQL major version number */
#define PG_MAJORVERSION_NUM 14

/* PostgreSQL minor version number */
#define PG_MINORVERSION_NUM 12

/* Define to best printf format archetype, usually gnu_printf if available. */
/* #undef PG_PRINTF_ATTRIBUTE */

/* Define to 1 to use <stdbool.h> to define type bool. */
#define PG_USE_STDBOOL 1

/* PostgreSQL version as a string */
#define PG_VERSION "14.12"

/* PostgreSQL version as a number */
#define PG_VERSION_NUM 140012

/* A string containing the version number, platform, and C compiler */
#define PG_VERSION_STR "PostgreSQL 14.12, compiled by Visual C++ build " CppAsString2(_MSC_VER) ", 64-bit"

/* Define to 1 to allow profiling output to be saved separately for each
   process. */
/* #undef PROFILE_PID_DIR */

/* Define to necessary symbol if this constant uses a non-standard name on
   your system. */
/* #undef PTHREAD_CREATE_JOINABLE */

/* RELSEG_SIZE is the maximum number of blocks allowed in one disk file. Thus,
   the maximum size of a single file is RELSEG_SIZE * BLCKSZ; relations bigger
   than that are divided into multiple files. RELSEG_SIZE * BLCKSZ must be
   less than your OS' limit on file size. This is often 2 GB or 4GB in a
   32-bit operating system, unless you have large file support enabled. By
   default, we make the limit 1 GB to avoid any possible integer-overflow
   problems within the OS. A limit smaller than necessary only means we divide
   a large relation into more chunks than necessary, so it seems best to err
   in the direction of a small limit. A power-of-2 value is recommended to
   save a few cycles in md.c, but is not absolutely required. Changing
   RELSEG_SIZE requires an initdb. */
#define RELSEG_SIZE 131072

/* The size of `bool', as computed by sizeof. */
#define SIZEOF_BOOL 1

/* The size of `long', as computed by sizeof. */
#define SIZEOF_LONG 4

/* The size of `off_t', as computed by sizeof. */
/* #undef SIZEOF_OFF_T */

/* The size of `size_t', as computed by sizeof. */
#define SIZEOF_SIZE_T 8

/* The size of `void *', as computed by sizeof. */
#define SIZEOF_VOID_P 8

/* Define to 1 if you have the ANSI C header files. */
#define STDC_HEADERS 1

/* Define to 1 if strerror_r() returns int. */
/* #undef STRERROR_R_INT */

/* Define to 1 to use ARMv8 CRC Extension. */
/* #undef USE_ARMV8_CRC32C */

/* Define to 1 to use ARMv8 CRC Extension with a runtime check. */
/* #undef USE_ARMV8_CRC32C_WITH_RUNTIME_CHECK */

/* Define to 1 to build with assertion checks. (--enable-cassert) */
/* #undef USE_ASSERT_CHECKING */

/* Define to 1 to build with Bonjour support. (--with-bonjour) */
/* #undef USE_BONJOUR */

/* Define to 1 to build with BSD Authentication support. (--with-bsd-auth) */
/* #undef USE_BSD_AUTH */

/* Define to build with ICU support. (--with-icu) */
#define USE_ICU 1

/* Define to 1 to build with LDAP support. (--with-ldap) */
#define USE_LDAP 1

/* Define to 1 to build with XML support. (--with-libxml) */
#define USE_LIBXML 1

/* Define to 1 to use XSLT support when building contrib/xml2.
   (--with-libxslt) */
#define USE_LIBXSLT 1

/* Define to 1 to build with LLVM based JIT support. (--with-llvm) */
/* #undef USE_LLVM */

/* Define to 1 to build with LZ4 support. (--with-lz4) */
#define USE_LZ4 1

/* Define to select named POSIX semaphores. */
/* #undef USE_NAMED_POSIX_SEMAPHORES */

/* Define to 1 to build with OpenSSL support. (--with-ssl=openssl) */
#define USE_OPENSSL 1

/* Define to 1 to build with PAM support. (--with-pam) */
/* #undef USE_PAM */

/* Define to 1 to use software CRC-32C implementation (slicing-by-8). */
/* #undef USE_SLICING_BY_8_CRC32C */

/* Define to 1 use Intel SSE 4.2 CRC instructions. */
/* #undef USE_SSE42_CRC32C */

/* Define to 1 to use Intel SSE 4.2 CRC instructions with a runtime check. */
#define USE_SSE42_CRC32C_WITH_RUNTIME_CHECK 1

/* Define to build with systemd support. (--with-systemd) */
/* #undef USE_SYSTEMD */

/* Define to select SysV-style semaphores. */
/* #undef USE_SYSV_SEMAPHORES */

/* Define to select SysV-style shared memory. */
/* #undef USE_SYSV_SHARED_MEMORY */

/* Define to select unnamed POSIX semaphores. */
/* #undef USE_UNNAMED_POSIX_SEMAPHORES */

/* Define to select Win32-style semaphores. */
#define USE_WIN32_SEMAPHORES 1

/* Define to select Win32-style shared memory. */
#define USE_WIN32_SHARED_MEMORY 1

/* Define to 1 if `wcstombs_l' requires <xlocale.h>. */
/* #undef WCSTOMBS_L_IN_XLOCALE */

/* Define WORDS_BIGENDIAN to 1 if your processor stores words with the most
   significant byte first (like Motorola and SPARC, unlike Intel). */
#if defined AC_APPLE_UNIVERSAL_BUILD
# if defined __BIG_ENDIAN__
#  define WORDS_BIGENDIAN 1
# endif
#else
# ifndef WORDS_BIGENDIAN
/* #  undef WORDS_BIGENDIAN */
# endif
#endif

/* Size of a WAL file block. This need have no particular relation to BLCKSZ.
   XLOG_BLCKSZ must be a power of 2, and if your system supports O_DIRECT I/O,
   XLOG_BLCKSZ must be a multiple of the alignment requirement for direct-I/O
   buffers, else direct I/O may fail. Changing XLOG_BLCKSZ requires an initdb.
   */
#define XLOG_BLCKSZ 8192



/* Number of bits in a file offset, on hosts where this is settable. */
/* #undef _FILE_OFFSET_BITS */

/* Define to 1 to make fseeko visible on some hosts (e.g. glibc 2.2). */
/* #undef _LARGEFILE_SOURCE */

/* Define for large files, on AIX-style hosts. */
/* #undef _LARGE_FILES */

/* Define to `__inline__' or `__inline' if that's what the C compiler
   calls it, or to nothing if 'inline' is not supported under any name.  */
#ifndef __cplusplus
#define inline __inline
#endif

/* Define to keyword to use for C99 restrict support, or to nothing if not
   supported */
#define pg_restrict __restrict

/* Define to the equivalent of the C99 'restrict' keyword, or to
   nothing if this is not supported.  Do not define if restrict is
   supported directly.  */
/* #undef restrict */
/* Work around a bug in Sun C++: it does not support _Restrict or
   __restrict__, even though the corresponding Sun C compiler ends up with
   "#define restrict _Restrict" or "#define restrict __restrict__" in the
   previous line.  Perhaps some future version of Sun C++ will work with
   restrict; if so, hopefully it defines __RESTRICT like Sun C does.  */
#if defined __SUNPRO_CC && !defined __RESTRICT
# define _Restrict
# define __restrict__
#endif

/* Define to how the compiler spells `typeof'. */
/* #undef typeof */
