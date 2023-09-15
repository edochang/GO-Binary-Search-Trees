# BST Comparison
## Parallelize Hash Operations
### Hash Operations
When parallelizing the Hash Operation, the parallelize implementation that spawns multiple Go routines is faster.

![GPU_Speedup_by_ImPerformance_Behavior_by_Number_of_Hash_Workersplementation](/markdown_assets/Performance_Behavior_by_Number_of_Hash_Workers.png)

In the graph below, we can see speedup as we spawn more go routines to an extent.  The most optimal speedup seems to be about 16 threads for both a 100 record of 1,000 trees per record and 100,000 record of 10 trees per record.

After 16 threads, we can observe the Speedup decreasing or the Hash Time increasing for both files.  Based on this observation, we can conclude that Go to a certain extent cannot manage goroutines well enough that you do not have to worry about how many threads to spawn anymore.  I speculate based on these observations, that the slowdown could be hardware bound as the thread scheduler switches from one go routine to another on the threads that have been allocated to the Go Program.  In other words, the overhead for spawning and managing all those Go routines to threads will become a performance trade off, as we can see more evidently with processing 100,000 records for only 10 trees per record with 100,000 Go routines.

![Speedup_by_Number_of_Hash_Workers](/markdown_assets/Speedup_by_Number_of_Hash_Workers.png)

### Hash and Hash Group Operations
For the channel implementation, we start with the faster configuration of 16 hash workers based on observation from the previous section.  Looking at the Coarse dataset, we can see improvement in the Hash and Hash Group execution time for both a channel implementation (n… hash workers to 1 data worker) and parallel thread + shared memory  implementation (n… hash workers to n… data workers, where hash workers = data workers).  For channel implementation, we observed a speed up of ~3.7.

$$ \frac{3.732}{1.00} = 3.732 speedup $$

For parallel thread + shared memory implementation, we observed similar speedup and observed that when we scale hash workers and data workers up, we will also see speedup improvements to scale.  See the chart below for the scaling behavior (the 6 blue bars from the right for Coarse dataset).

> **Parallel Thread + Shared Memory Implementation Notes:**  Parallel Thread + Shared Memory Implementation – Uses the same thread / go routine to hash and do Hash Group Operations.  The read from shared memory for trees and updating the hash value is guarded by data decomposition, where multiple threads cannot read and write from the same tree index.  The write to Hash Group Map is guarded by a mutex lock to handle concurrent write but will have lock contentions as an overhead.

When comparing the channel implementation, the parallel thread + shared memory implementation, and sequential implementation, the overhead to enable parallelization is more noticeable with the channel implementation with the Fine dataset.  With 100,000 trees to be processes / communicated between the Hash Worker and Data Worker, we can observe the bottleneck that the channel creates, even when the channel is enabled for buffering .  When compared to the parallel thread + shared memory implementation, the parallel thread implementation does better without the channel bottleneck.  However, the parallel thread implementation is still slower than the sequential implementation.  I speculate that the mutex lock contention becomes an overhead and bottleneck to gain speedup against the sequential implementation especially, since 100,000 Tree IDs needs to be registered to the Shared Memory – Hash Group Map that all the threads need to write to.  This can cause the threads to wait on each other until each thread has written into the Map and release the mutex lock.

At first, I found the parallel thread implementation simpler, because its parallel programming paradigm is like what we have implemented in previous labs for balanced trees in Pre-Fix Sum and KMean CUDA programming, when we used both data decomposition and mutex lock / atomic operations design patterns to enable more parallelism / concurrency.  After understanding the behaviors of channels, its synchronization capabilities, and constructs, the simpler channel design patterns were easier to understand.   However, as the programmer gain familiarity to channels and its programming paradigm, I speculate the implementation will not steer programmers from using channels to expose more parallelism / concurrency.

![Hash_and_Hash_Group_Performance_by_Number_of_Hash_Workers_to_Data_Workers](/markdown_assets/Hash_and_Hash_Group_Performance_by_Number_of_Hash_Workers_to_Data_Workers.png)

Using the chart below, another observation worth calling out when we scale the channel implementation by Hash Workers, is that for smaller tree volumes in the Coarse data set (100 trees), the channel implementation does scale up to all the trees where each hash worker is only working on 1 tree and sending the hash value to the data worker.  It evens scales well against the sequential implementation.  However, if we look at the Fine data set (100,000 trees), we start to observe the bottleneck that the single channel to data worker creates.  It becomes more obvious when we increase the Hash Workers to 1000, even with a buffered channel.  We start to see the synchronization behaviors that the channel offers becomes a bottle neck.

See a quote from Go’s documentation on explaining the synchronization behavior that the channel offers, when blocking sender / receivers with un-buffer and buffer channels.  I speculate this feature / capability you get from channels becomes a bottleneck when the volume of data has a direct relationship to what you are sending in the channel.

> _“Receivers always block until there is data to receive. If the channel is unbuffered, the sender blocks until the receiver has received the value. If the channel has a buffer, the sender blocks only until the value has been copied to the buffer; if the buffer is full, this means waiting until some receiver has retrieved a value._
> 
> _A buffered channel can be used like a semaphore, for instance to limit throughput. In this example, incoming requests are passed to handle, which sends a value into the channel, processes the request, and then receives a value from the channel to ready the “semaphore” for the next consumer. The capacity of the channel buffer limits the number of simultaneous calls to process.”_

Reference: https://go.dev/doc/effective_go#channels

See the chart below visualizing the channel implementation’s execution time when we scale Hash Workers to 1 Data Worker to support the behaviors observed and stated in the above section.

![Hash_and_Hash_Group_Performance_by_Number_of_Hash_Workers_to_1_Data_Worker](/markdown_assets/Hash_and_Hash_Group_Performance_by_Number_of_Hash_Workers_to_1_Data_Worker.png)

### Optional Implementation

The access to the shared data structure with a single mutex lock on the map is still a bottleneck as well as the single channel to communicate many hash workers to data workers.  However, having more than one data worker does help reduce the effects of the single channel bottleneck, since there’s more than 1 reader removing items from the channel unblocking any potentially blocked senders.  If we compare from the chart below the 16-1 time with the 16-14 time (1.074), we can observe the performance to be slightly worse, but not by a lot.  I speculate from this behavior, that if the spread between the number of hash workers and data workers is small, that the effects of a single channel bottleneck can be reduced.  If the sender and receiver become more symmetrical on both sides, then it will minimize the effects of a threads potentially blocked on the channel due to each threads spending too much time processing their computation before able to read the next item in the channel.  However, minimizing this effect will unfortunately increase the effects of lock contention on the shared Hash Map.  

In my next version of the optional implementation, I spread the map lock with a slice lock.  When a hash group doesn’t exist, lock the map to create it.  If it exists, then only lock the array and not the map.  My observations are in orange in the two charts below.  I saw improvements in the execution time, when I spread the lock contention on the shared memory across other shared memory objects (implementing “lock granularity”).

![Hash_and_Hash_Group_Performance_where_Hash_Workers-Data_Workers_and_Both_Are_Not_1_1](/markdown_assets/Hash_and_Hash_Group_Performance_where_Hash_Workers-Data_Workers_and_Both_Are_Not_1_1.png)

However, I observed worse performance for hybrid coarse and fine grain locking with 100,000 trees dataset.  I speculate that the shear large volume of trees which had a lot of hash collision shifted the lock contention from one granularity to another.  The resulting hash groups for 100,000 trees gives you 1,000 groups.  That means there was a lot of read / writes on the map and on the slice, to create the 1,000 hash groups.  I didn’t see any improvements in parallelism because of this compared to just the map lock implementation.  The single channel and lock granularity in general didn’t perform well with the sequential because of the channel bottleneck and lock contentions.

![Hash_and_Hash_Group_Performance_where_Hash_Workers-Data_Workers_and_Both_Are_Not_1_2](/markdown_assets/Hash_and_Hash_Group_Performance_where_Hash_Workers-Data_Workers_and_Both_Are_Not_1_2.png)

Unlike the Fine dataset, the Coarse dataset can observe performance benefits with the m:n workers on a single channel with a hybrid lock.  In addition, the Coarse data set produced only 2 hash groups, which shifted most of the lock contention on the slice, showing the benefits of the granular slice lock per hash group.

![Hash_and_Hash_Group_Performance_by_Central_Manager](/markdown_assets/Hash_and_Hash_Group_Performance_by_Central_Manager.png)
![Hash_and_Hash_Group_Performance_by_Unequal_Hash_Workers_and_Data_Workers](/markdown_assets/Hash_and_Hash_Group_Performance_by_Unequal_Hash_Workers_and_Data_Workers.png)

## Parallelize Tree Comparison
For using a data structure that enabled safe write across concurrent threads, we observe that parallel tree comparisons write results to an adjacency matrix performs better with no locks on the Coarse dataset.  Even though the logic only allows a go-routine to update a specific location in the array, I chose to add a lock to see how much lock contention would impact the performance.  This met my expectation that the lock contention will have a negative impact to performance.

![Adjacency_Matrix_Compare_Tree_Performance_Coarse_Lock](/markdown_assets/Adjacency_Matrix_Compare_Tree_Performance_Coarse_Lock.png)
![Adjacency_Matrix_Compare_Tree_Performance_Fine_Lock](/markdown_assets/Adjacency_Matrix_Compare_Tree_Performance_Fine_Lock.png)

Note: Fine with lock couldn’t be executed.  The Go Program killed itself, hence the execution time is 0 ms.  However, the no-lock implementation was able to be processed.  Again, the behavior matches to what I observed and explained for the Coarse dataset.

With that said, my buffered implementation did not perform well against the first implementation because I chose a data structure that didn’t need to be filled with false values that an adjacency array / matrix needed.  Because I can’t guarantee a thread will write only to a specific address space, I implemented a lock to handle read / write contention on the sparse/array slice data structure I created.  I observed my buffered implementation’s performance did perform better than adjacency matrix with lock performance, however, as I increased the comp workers closer to the number of trees, the lock contention on the queue using conditional locks and the lock on the sparse slices in the map were creating performance overhead.

The performance observed for Parallelize Tree Comparison does not scale against the sequential implementation for both Coarse and Fine data sets.  It does scale to a certain extent against comp workers for parallel implementation, but it will run into a bottleneck like Go’s Channels.

![Tree_Comparison_Performance_by_Number_of_Comp_Workers_Coarse](/markdown_assets/Tree_Comparison_Performance_by_Number_of_Comp_Workers_Coarse.png)
![Tree_Comparison_Performance_by_Number_of_Comp_Workers_Fine](/markdown_assets/Tree_Comparison_Performance_by_Number_of_Comp_Workers_Fine.png)

The complexity of implementing parallelism by taking advantage of comparing each tree in the Hash Groups while doing it efficiently with less work is more complicated, because the tree comparison with tree pairs does not handle a comparison dependency where, ideally, you should not compare trees that have been compared before.  This will require the workers and producer to communicate and not compare a tree pair that has been visited already.
In summary two dependencies needs to be managed:
1. Producer or worker needs to know when trees have been compared before (1, 3 == 3, 1)
2. Producer knows when new comparison work needs to be provided when subgroups of trees within a Hash Groups are different as identified by the workers.  In other words, identify whether the tree should be in the existing Hash Group or should be moved to another Group.  

Without managing these dependencies, then the matrix below visually depicts the duplicate work an adjacency matrix or sparse adjacency matrix creates and limits the value of parallel processing as compared to sequential processing.

![Problem_Structure_1](/markdown_assets/Problem_Structure_1.png)

For example, in the sequential implementation it only visits and compares trees once and creates new Comparison Groups when needed within a Hash Group.

There is opportunity to further improve my parallel implementation to reduce the amount of duplicate work.  For example, if I remove the additional colored items (trees) to compare in the bottom left of the diagonal.  See the visual below as an example.

![Problem_Structure_2](/markdown_assets/Problem_Structure_2.png)

If I can achieve this, we can further parallelize the creation of the comparison groups away from this sparse structure below.

![Problem_Structure_3](/markdown_assets/Problem_Structure_3.png)

Doing these suggested items, to ultimately manage the two dependencies called out should improve the parallel execution time further. 

However, I speculate this wouldn’t beat my sequential implementation still, given the overhead to communicate threads using a thread-safe queue-like data structure, handling shared memory contention with locks at the queue and group data structure level, and the overhead of thread scheduling against too many go routines when you scale up. 

I speculate and believe the additional complexity of managing a thread pool outside of Go’s channel’s implementation does not seem to be worthwhile for this use case (Comparing Tree Pairs as part of Tree Comparison).

# Appendix
## Computer Information
AMD Ryzen 9 5900HS with Radeon Graphics, 3301 Mhz, 8 Core(s), 16 Logical Processor(s)
Installed Physical Memory (RAM): 24.0 GB

### lscpu on WSL2 ###

![lscpu](/markdown_assets/lscpu.png)

### OS Version ###
Windows 11 Pro, Version 22H2, OS Build 22621.608
Windows Subsystem for Linux Distributions:  Ubuntu-20.04

    uname -r >>> 5.15.57.1-microsoft-standard-WSL2

## Running The Program
Run the following in the terminal:

    ./bin/BST -input input/simple12.txt -hash-workers 1

**./bin/BST Argument Instructions**

    ./bin/BST -h
    Usage of ./bin/BST:
    -comp-workers int
            integer-value number of threads
    -data-workers int
            integer-value number of threads
    -hash-workers int
            integer-value number of threads
    -input string
            string-valued path to an input file
    -print-limit
            Existence of the flag limits printing more than 300 groups for HashGroups and ComparisonGroups.
    -single-trees
            Existence of the flag prints single trees
