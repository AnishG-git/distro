## Core Functionality
- 
-


## Optimizations/Organization improvements
- Connection pooling
- Better keepalive, healthcheck mechanism
- Better separation of concerns
- Resource-aware job scheduling system
    - Inputs for scheduling decisions:
        - Available RAM on the server
        - Estimated time for each job
        - Maximum number of concurrent jobs
    - Streaming job processing capabilities:
        - Real-time job acceptance and evaluation
        - Dynamic queue management for incoming jobs
        - Adaptive resource allocation as jobs complete
    - Scheduling algorithms:
        - Priority determination based on resource availability
        - Time-aware scheduling to optimize throughput
        - Load balancing for concurrent execution
    - Performance optimizations:
        - Resource reservation for predictable execution
        - Dynamic adjustment of concurrency based on system load
        - Efficient cleanup and resource release after job completion

- Streaming encoding and sharding