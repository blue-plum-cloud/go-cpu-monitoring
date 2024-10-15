#include "perf_api.h"

void reset_and_enable_ioctl(int fd){
    int err = ioctl(fd, PERF_EVENT_IOC_RESET,PERF_IOC_FLAG_GROUP);
    if (err){
        perror("reset_and_enable_ioctl");
        exit(EXIT_FAILURE);
    }
    err = ioctl(fd, PERF_EVENT_IOC_ENABLE, PERF_IOC_FLAG_GROUP);
    if (err){
        perror("reset_and_enable_ioctl");
        exit(EXIT_FAILURE);
    }
}

void disable_ioctl(int fd){
    int err = ioctl(fd, PERF_EVENT_IOC_DISABLE, PERF_IOC_FLAG_GROUP);
    if (err){
        perror("reset_and_enable_ioctl");
        exit(EXIT_FAILURE);
    }
}

void config_perf(struct perf_event_attr *pe,int *fd,uint64_t type, uint64_t config){
    // memset(pe, 0, sizeof(*pe));
    pe->type = type;
    pe->size = sizeof(*pe);
    pe->config = config;
    pe->disabled = 1;
    pe->exclude_kernel = 1;
    pe->exclude_hv = 1;
    *fd = syscall(__NR_perf_event_open, pe, 0, -1, -1, 0);
}

uint64_t config_cache_id(uint64_t perf_hw_cache_id, uint64_t perf_hw_cache_op_id, uint64_t perf_hw_cache_op_result_id){
    return (perf_hw_cache_id) | 
    (perf_hw_cache_op_id << 8) | 
    (perf_hw_cache_op_id << 16);
}

/*
The group_fd argument allows event groups to be created. An
event group has one event which is the group leader. The leader
is created first, with group_fd = -1. The rest of the group
members are created with subsequent perf_event_open() calls with
group_fd being set to the file descriptor of the group leader.
(A single event on its own is created with group_fd = -1 and is
considered to be a group with only 1 member.) An event group is
scheduled onto the CPU as a unit: it will be put onto the CPU
only if all of the events in the group can be put onto the CPU.
This means that the values of the member events can be
meaningfully compared—added, divided (to get ratios), and so on—
with each other, since they have counted events for the same set
of executed instructions.

Inputs:
struct perf_event_attr pe;
int fds[NUM_EVENTS];       // Array for file descriptors
uint64_t ids[NUM_EVENTS];  // Array for IDs
uint64_t types[NUM_EVENTS];
uint64_t configs[NUM_EVENTS];
*/
int config_perf_multi(struct perf_event_attr *pe, 
                        int *fds, 
                        uint64_t *ids, 
                        uint64_t *types, 
                        uint64_t *configs,
                        int event_count,
                        int cpu) {
    int group_fd = -1;

    for (int i = 0; i < event_count; i++) {
        // Initialize the event attributes for each event
        memset(pe, 0, sizeof(*pe));
        pe->type = types[i];
        pe->size = sizeof(*pe);
        pe->config = configs[i];
        pe->disabled = 1; // Start disabled
        pe->exclude_kernel = 1;
        pe->exclude_hv = 1;
        pe->read_format = PERF_FORMAT_GROUP | PERF_FORMAT_ID;
 
        // Open the first event
        if (i == 0) {
            //int syscall(SYS_perf_event_open, struct perf_event_attr *attr,
            //pid_t pid, int cpu, int group_fd, unsigned long flags);
            group_fd = syscall(__NR_perf_event_open, pe, -1, cpu, -1, 0);
            if (group_fd < 0) {
                perror("perf_event_open");
                exit(EXIT_FAILURE);
            }
            ioctl(group_fd, PERF_EVENT_IOC_ID, &ids[i]);
        } else {
            // For subsequent events, add to the existing group
            fds[i] = syscall(__NR_perf_event_open, pe, -1, cpu, group_fd, 0);
            if (fds[i] < 0) {
                perror("perf_event_open");
                exit(EXIT_FAILURE);
            }
            ioctl(fds[i], PERF_EVENT_IOC_ID, &ids[i]);
        }

    }
    return group_fd;
}

void something(){
    // Perform some work
    for (volatile int i = 0; i < 1000000; i++); // Busy-wait loop
}

void get_perf(struct perf_event_attr *pe, char ** string, uint64_t *ids, int event_count, int group_fd, int *values){

    // parse perf
    size_t buffer_size = sizeof(struct read_format)+sizeof(struct value)*event_count;
    char * buf = malloc(buffer_size);
    struct read_format * rfs = (struct read_format*) buf;
    ssize_t bytes_read = read(group_fd, buf, buffer_size);
    if (bytes_read < 0) {
        free(buf);
        perror("read");
        exit(EXIT_FAILURE);
    }


    for (int i = 0; i < event_count; i++){
        for (int j = 0; j < event_count; j++){
            if (rfs->values[i].id==ids[j]){
                printf("%s: %lu\n", string[i],rfs->values[i].value);
                values[i] = rfs->values[i].value;
            }
        }
    }

    free(buf);
}

