package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"utexas/cs380p/binarysearchtrees/helper"
)

// global variables
var sliceTrees_g []*Tree
var mapTrees_g = make(map[int]*Tree)
var hashGroups = make(map[int][]int)
var hashGroupsOpt = make(map[int]*HashGroup)
var mw io.Writer

func main() {
	logFile, errLog := os.OpenFile("log.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if errLog != nil {
		panic(errLog)
	}

	if err := os.Truncate("log.txt", 0); err != nil {
		log.Printf("Failed to truncate: %v", err)
	}

	mw = io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	// Define and bind the flag
	intHashWorker := flag.Int("hash-workers", 0, "integer-value number of threads")
	intDataWorker := flag.Int("data-workers", 0, "integer-value number of threads")
	intCompWorker := flag.Int("comp-workers", 0, "integer-value number of threads")
	stringInput := flag.String("input", "", "string-valued path to an input file")
	boolSingleTrees := flag.Bool("single-trees", false, "Existence of the flag prints single trees")
	boolPrintLimit := flag.Bool("print-limit", false, "Existence of the flag limits printing more than 300 groups for HashGroups and ComparisonGroups.")

	flag.Parse()

	fmt.Fprintf(mw, "hash-workers: %v  -:-  ", *intHashWorker)
	fmt.Fprintf(mw, "data-workers: %v  -:-  ", *intDataWorker)
	fmt.Fprintf(mw, "comp-workers: %v  -:-  ", *intCompWorker)
	fmt.Fprintf(mw, "input: %v  -:-  ", *stringInput)
	fmt.Fprintf(mw, "single-trees: %v  -:-  ", *boolSingleTrees)
	fmt.Fprintf(mw, "print-limit: %v\n", *boolPrintLimit)

	_, err := filepath.Abs(*stringInput)
	fileAbsPath, err := filepath.Abs(*stringInput)
	helper.Check(err)
	fmt.Fprintf(mw, "Absolute path of input is: %v\n", fileAbsPath) // debug statement

	fileLines := helper.ReadFile(*stringInput)
	sliceTrees_g = make([]*Tree, 0)
	processFileLinesAndCreateTrees(fileLines)

	// book keeping
	noLimit := !*boolPrintLimit
	var start time.Time
	var elapsed time.Duration
	var orderedHashGroupsKeys []int

	// concurrency variables
	var cTreeIdx chan int
	wg := &sync.WaitGroup{}

	// Hash Time Section
	if *intHashWorker > 1 && *intDataWorker == 0 && *intCompWorker == 0 {
		fmt.Fprintf(mw, "Entered operation for hash-workers(%v)\n", *intHashWorker) // debug statement
		start = time.Now()
		hashTreesGoPart1(*intHashWorker, wg)
		elapsed = time.Since(start)
		helper.PrintTime(elapsed, "hashTime", &mw)
	}
	if *intHashWorker <= 1 {
		fmt.Fprintf(mw, "Entered operation for hash-workers(%v)\n", *intHashWorker) // debug statement
		start = time.Now()
		hashTrees()
		elapsed = time.Since(start)
		helper.PrintTime(elapsed, "hashTime", &mw)
	}

	// Hash Group Section
	if *intHashWorker <= 1 && *intDataWorker == 1 {
		fmt.Fprintf(mw, "Entered operation for hash-workers(%v) and data-workers(%v) - sequential\n", *intHashWorker, *intDataWorker) // debug statement
		createHashGroups()
		elapsed = time.Since(start)
		helper.PrintTime(elapsed, "hashGroupTime", &mw)

		orderedHashGroupsKeys = getOrderedHashKeys()

		//printHashGroupsWithKeys(orderedHashGroupsKeys)

		printHashGroupsWithSortedKeys(orderedHashGroupsKeys, noLimit, *boolSingleTrees)
	}
	if *intHashWorker > 1 && *intDataWorker == 1 {
		// hash workers needs to send on a channel to the central manager data worker
		fmt.Fprintf(mw, "Entered operation for hash-workers(%v) and data-workers(%v) - central manager\n", *intHashWorker, *intDataWorker) // debug statement
		//centralManagerDone := make(chan bool)
		cTreeIdx = make(chan int, *intHashWorker)

		start = time.Now()
		hashTreesGoPart2(*intHashWorker, cTreeIdx, wg)
		go monitorHashTreesGo(wg, cTreeIdx, &start)
		createHashGroupsGoPart2(cTreeIdx)
		elapsed = time.Since(start)
		helper.PrintTime(elapsed, "hashGroupTime", &mw)
		//go createHashGroupsGoPart2(cTreeIdx, centralManagerDone)

		/*
			if <-centralManagerDone {
				elapsed = time.Since(start)
				helper.PrintTime(elapsed, "hashGroupTime")
			} else {
				fmt.Println("Program ran into an error waiting for Central Manager Go Routine (data-workers=1).")
			}
		*/

		time.Sleep(1 * time.Millisecond) // sleep to give the hashtime time to print before printing the results.

		sortHashGroupsContent()
		orderedHashGroupsKeys = getOrderedHashKeys()
		printHashGroupsWithSortedKeys(orderedHashGroupsKeys, noLimit, *boolSingleTrees)
	}
	if *intHashWorker > 1 && *intHashWorker == *intDataWorker {
		fmt.Fprintf(mw, "Entered operation for hash-workers(%v) and data-workers(%v) - hash and data worker same thread\n", *intHashWorker, *intDataWorker) // debug statement
		start = time.Now()
		hashTreesAndGroupsGoPart3(*intHashWorker, wg)
		elapsed = time.Since(start)
		helper.PrintTime(elapsed, "hashTime", &mw)
		helper.PrintTime(elapsed, "hashGroupTime", &mw)

		sortHashGroupsContent()
		orderedHashGroupsKeys = getOrderedHashKeys()
		printHashGroupsWithSortedKeys(orderedHashGroupsKeys, noLimit, *boolSingleTrees)
	}

	// Optional Implementation Here
	/*
		if *intDataWorker > 1 && *intHashWorker != *intDataWorker {
			wg2 := &sync.WaitGroup{}
			fmt.Fprintf(mw, "Entered operation for hash-workers(%v) and data-workers(%v) - optional implementation\n", *intHashWorker, *intDataWorker) // debug statement
			cTreeIdx = make(chan int, *intHashWorker)

			start = time.Now()
			hashTreesGoPart2(*intHashWorker, cTreeIdx, wg)
			go monitorHashTreesGo(wg, cTreeIdx, &start)
			createHashGroupsGoPartOpt(cTreeIdx, wg2, *intDataWorker)
			wg2.Wait()
			elapsed = time.Since(start)
			helper.PrintTime(elapsed, "hashGroupTime", &mw)

			sortHashGroupsContent()
			orderedHashGroupsKeys = getOrderedHashKeys()
			printHashGroupsWithSortedKeys(orderedHashGroupsKeys, noLimit, *boolSingleTrees)
		}
	*/
	// optional implementation with a hybrid coarse and fine grain lock - worst performance than a general map lock as coarse.
	if *intDataWorker > 1 && *intHashWorker != *intDataWorker {
		wg2 := &sync.WaitGroup{}
		fmt.Fprintf(mw, "Entered operation for hash-workers(%v) and data-workers(%v) - optional implementation\n", *intHashWorker, *intDataWorker) // debug statement
		cTreeIdx = make(chan int, *intHashWorker)

		start = time.Now()
		hashTreesGoPart2(*intHashWorker, cTreeIdx, wg)
		go monitorHashTreesGo(wg, cTreeIdx, &start)
		createHashGroupsGoPartOpt2(cTreeIdx, wg2, *intDataWorker)
		wg2.Wait()
		elapsed = time.Since(start)
		helper.PrintTime(elapsed, "hashGroupTime", &mw)

		convertHashGroupsOptToHashGroups()

		sortHashGroupsContent()
		orderedHashGroupsKeys = getOrderedHashKeys()
		printHashGroupsWithSortedKeys(orderedHashGroupsKeys, noLimit, *boolSingleTrees)
	}

	// Tree Comparison Group Section
	if *intCompWorker == 1 {
		orderedHashGroupsKeys = getOrderedHashKeys()

		start = time.Now()
		comparisonGroups := createComparisonGroups(orderedHashGroupsKeys)
		//comparisonGroups := createComparisonGroups()
		//fmt.Println(comparisonGroups) // debug statement
		elapsed = time.Since(start)
		helper.PrintTime(elapsed, "compareTreeTime", &mw)

		printComparisonGroups(&comparisonGroups, noLimit)
	}

	if *intCompWorker > 1 {
		start = time.Now()
		equalTrees := createComparisonGroupsGo(*intCompWorker, wg)
		elapsed = time.Since(start)
		helper.PrintTime(elapsed, "compareTreeTime", &mw)

		comparisonGroups := convertToComparisonGroups(equalTrees)

		printComparisonGroups(comparisonGroups, noLimit)
	}
	if *intCompWorker == -1 {
		orderedHashGroupsKeys = getOrderedHashKeys()

		start = time.Now()
		adjMatrix := part3FirstImplementation(orderedHashGroupsKeys, wg)
		elapsed = time.Since(start)
		helper.PrintTime(elapsed, "adjMatrixCompareTreeTime", &mw)

		printPart3FirstImplementation(adjMatrix, &mw, noLimit)
	}
}

// //////////////////////////////// Tree ///////////////////////////////
type Tree struct {
	treeId         int
	topNode        *Node
	inOrderContent []int
	treeHash       int
}

type Node struct {
	key   int
	value int
	left  *Node //left
	right *Node //right
}

func processFileLinesAndCreateTrees(input []string) {
	for i := 0; i < len(input); i++ {
		lineSplit := strings.Split(input[i], " ")
		tempTree := createTree(lineSplit, i)
		sliceTrees_g = append(sliceTrees_g, &tempTree)
		mapTrees_g[tempTree.treeId] = &tempTree
		// fmt.Println(lineSplit)
	}
}

func createTree(inputs []string, id int) (tree Tree) {
	//generatedTreeId := "id" + fmt.Sprintf("%02d", id)
	for i := 0; i < len(inputs); i++ {
		stringValue, _ := strconv.Atoi(inputs[i])
		node := &Node{
			key:   i,
			value: stringValue,
			left:  nil,
			right: nil,
		}
		if tree.topNode == nil {
			//emptySlice := make([]int, 0)
			tree = Tree{
				treeId:         id,
				topNode:        node,
				inOrderContent: nil,
			}
		} else {
			insertNodeToTree(node, tree.topNode)
		}
	}

	tree.inOrderContent = tree.inOrderTraverse()

	//tree.treeHash = tree.hashTree()
	//fmt.Printf("hashTree (%s) Value: %d \n", tree.treeId, tree.treeHash) // debug statement

	return tree
}

func insertNodeToTree(insertNode *Node, node *Node) {
	//fmt.Printf("node value (%d) and insert node value (%d)\n", node.value, insertNode.value)  debug statement
	if insertNode.value < node.value {
		if node.left == nil {
			node.left = insertNode
		} else {
			insertNodeToTree(insertNode, node.left)
		}
	} else {
		if node.right == nil {
			node.right = insertNode
		} else {
			insertNodeToTree(insertNode, node.right)
		}
	}
}

func (tree *Tree) inOrderTraverse() (result []int) {
	result = inOrderTraverse(tree.topNode)

	return result
}

func inOrderTraverse(node *Node) (result []int) {
	if node == nil {
		return make([]int, 0)
	}

	result = inOrderTraverse(node.left)
	result = append(result, node.value)
	//fmt.Printf("%d ", node.value) // debug statement
	result = append(result, inOrderTraverse(node.right)...)
	return result
}

func (tree *Tree) printTreeContent() {
	fmt.Printf("Item(%d): ", tree.treeId)
	content := tree.inOrderTraverse()
	for _, item := range content {
		fmt.Printf("%d ", item)
	}
	fmt.Println()
}

func (tree *Tree) hashTree() int {
	// initial hash value
	hash := 1
	//fmt.Println(tree.inOrderContent) // debug statement
	for _, value := range tree.inOrderContent {
		new_value := value + 2
		hash = (hash*new_value + new_value) % 1000
		//fmt.Printf("Tree Value: %d ; hash Value: %d \n", value, hash) // debug statement
	}
	return hash
}

func hashTrees() {
	for i := range sliceTrees_g {
		sliceTrees_g[i].treeHash = sliceTrees_g[i].hashTree()
		//fmt.Printf("sliceTrees_g(%d): %d \n", i, sliceTrees_g[i].treeHash) // debug statement
	}
}

func hashTreesGoPart1(workers int, wg *sync.WaitGroup) {
	itemPerWorker := len(sliceTrees_g) / workers
	//fmt.Printf("hashTreesGoPart1: itemPerWorker: %v \n", itemPerWorker)

	if itemPerWorker == 0 {
		itemPerWorker = 1
		workers = len(sliceTrees_g)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerId := i
		go func() {
			var startIdx, endIdx int
			startIdx = workerId * itemPerWorker
			if workerId == workers-1 {
				endIdx = len(sliceTrees_g)
			} else {
				endIdx = startIdx + itemPerWorker
			}

			//fmt.Printf("Worker(%d): startIdx: %d, endIdx: %d \n", workerId, startIdx, endIdx) // debug statement
			for treeIdx := startIdx; treeIdx < endIdx; treeIdx++ {
				//fmt.Printf("Worker(%d): treeIdx: %d \n", workerId, treeIdx) // debug statement
				hashValue := sliceTrees_g[treeIdx].hashTree()
				sliceTrees_g[treeIdx].treeHash = hashValue
				//fmt.Printf("workerId(%d): sliceTrees_g(%d): %d \n", workerId, treeIdx, sliceTrees_g[treeIdx].treeHash) // debug statement
			}
			wg.Done()
		}()
	}

	wg.Wait()
}

func hashTreesGoPart2(workers int, cTreeIdx chan int, wg *sync.WaitGroup) {
	lenSliceTreesG := len(sliceTrees_g)
	itemPerWorker := lenSliceTreesG / workers

	if itemPerWorker == 0 {
		itemPerWorker = 1
		workers = lenSliceTreesG
	}

	//fmt.Printf("hashTreesGoPart2.2: workers(%v) itemPerWorker: %v \n", workers, itemPerWorker)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(workerId int, cTreeIdx chan<- int, lenSliceTreesG int, wg *sync.WaitGroup) {
			defer wg.Done()
			var startIdx, endIdx int
			startIdx = workerId * itemPerWorker
			if workerId == workers-1 {
				endIdx = lenSliceTreesG
			} else {
				endIdx = startIdx + itemPerWorker
			}

			//fmt.Printf("Worker(%d): startIdx: %d, endIdx: %d \n", workerId, startIdx, endIdx) // debug statement
			for treeIdx := startIdx; treeIdx < endIdx; treeIdx++ {
				//fmt.Printf("Worker(%d): treeIdx: %d \n", workerId, treeIdx) // debug statement
				hashValue := sliceTrees_g[treeIdx].hashTree()
				sliceTrees_g[treeIdx].treeHash = hashValue
				//fmt.Printf("workerId(%d): sliceTrees_g(%d): %d \n", workerId, treeIdx, sliceTrees_g[treeIdx].treeHash) // debug statement

				//fmt.Printf("Send to cTreeIdx channel: treeIdx(%d) \n", treeIdx) // debug statement placeholder
				cTreeIdx <- treeIdx
			}
		}(i, cTreeIdx, lenSliceTreesG, wg)
	}
}

func monitorHashTreesGo(wg *sync.WaitGroup, cTreeIdx chan int, start *time.Time) {
	//fmt.Println("Waiting...") // debug statement
	wg.Wait()
	//fmt.Println("NOT WAITING! GO!") // debug statement
	elapsed := time.Since(*start)
	helper.PrintTime(elapsed, "hashTime", &mw)

	close(cTreeIdx)
}

func hashTreesAndGroupsGoPart3(workers int, wg *sync.WaitGroup) {
	itemPerWorker := len(sliceTrees_g) / workers
	//fmt.Printf("hashTreesGoPart2.3: itemPerWorker: %v \n", itemPerWorker)

	if itemPerWorker == 0 {
		itemPerWorker = 1
		workers = len(sliceTrees_g)
	}

	hashGroups = make(map[int][]int)
	m := sync.Mutex{}
	lenSliceTreesG := len(sliceTrees_g)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerId int, m *sync.Mutex, lenSliceTreesG int) {
			var startIdx, endIdx int
			startIdx = workerId * itemPerWorker
			if workerId == workers-1 {
				endIdx = lenSliceTreesG
			} else {
				endIdx = startIdx + itemPerWorker
			}

			//fmt.Printf("Worker(%d): startIdx: %d, endIdx: %d \n", workerId, startIdx, endIdx) // debug statement
			for treeIdx := startIdx; treeIdx < endIdx; treeIdx++ {
				//fmt.Printf("Worker(%d): treeIdx: %d \n", workerId, treeIdx) // debug statement
				hashValue := sliceTrees_g[treeIdx].hashTree()
				sliceTrees_g[treeIdx].treeHash = hashValue
				//fmt.Printf("workerId(%d): sliceTrees_g(%d): %d \n", workerId, treeIdx, sliceTrees_g[treeIdx].treeHash) // debug statement

				// Note: as you build more concurrency in, it gets more and more difficult to create an ordered HashGroup, unless specific care / logic is written to get the right order, which could slow performance and is not the intent of the lab.
				itemTree := sliceTrees_g[treeIdx]

				m.Lock()
				hashGroups[itemTree.treeHash] = append(hashGroups[itemTree.treeHash], itemTree.treeId)
				m.Unlock()
			}
			wg.Done()
		}(i, &m, lenSliceTreesG)
	}
	wg.Wait()
}

// /////////////////////////////// Hash Groups ////////////////////////////
type HashGroup struct {
	potTwins []int
	lock     *sync.Mutex
}

func createHashGroups() {
	//start := time.Now()

	for _, itemTree := range sliceTrees_g {
		//for _, itemTree := range mapTrees_g {
		//itemTree.printTreeContent()
		hashGroups[itemTree.treeHash] = append(hashGroups[itemTree.treeHash], itemTree.treeId)
	}

	//elapsed := time.Since(start)
	//helper.PrintTime(elapsed, "hashGroupMethodTime")
}

func getOrderedHashKeys() (hashGroupsKey []int) {
	// create a slice of Keys.  Keys are the hashValue in HashGroup.
	for hashGroupKey := range hashGroups {
		hashGroupsKey = append(hashGroupsKey, hashGroupKey)
	}
	sort.Ints(hashGroupsKey)

	//fmt.Println(hashGroups) // debug statement

	return hashGroupsKey
}

func sortHashGroupsContent() {
	for key := range hashGroups {
		sort.Ints(hashGroups[key])
	}
}

func printHashGroupsWithSortedKeys(hashGroupsKey []int, noLimit bool, singleTrees bool) {
	if len(hashGroupsKey) < 300 || noLimit {
		for key := range hashGroupsKey {
			//fmt.Printf("twins length: %d: ", len(hashGroups[hashGroupValue].twins)) // debug statement
			if !singleTrees {
				if len(hashGroups[hashGroupsKey[key]]) > 1 {
					fmt.Fprintf(mw, "%d: ", hashGroupsKey[key])
					for _, treeId := range hashGroups[hashGroupsKey[key]] {
						fmt.Fprintf(mw, "%d ", treeId)
					}
					fmt.Fprintln(mw)
				}
			} else {
				fmt.Fprintf(mw, "%d: ", hashGroupsKey[key])
				for _, treeId := range hashGroups[hashGroupsKey[key]] {
					fmt.Fprintf(mw, "%d ", treeId)
				}
				fmt.Fprintln(mw)
			}
		}
	} else {
		fmt.Printf("hashGroup size: %d \n", len(hashGroupsKey))
	}
}

func printHashGroups(noLimit bool, singleTrees bool) {
	if len(hashGroups) < 300 || noLimit {
		//fmt.Printf("hashGroupsWithValueKey length: %d \n", len(hashGroupsWithValueKey)) // debug statement
		for key := range hashGroups {
			//fmt.Printf("twins length: %d: ", len(hashGroupsWithValueKey[key].potTwins)) // debug statement
			if !singleTrees {
				if len(hashGroups[key]) > 1 {
					fmt.Fprintf(mw, "%d: ", key)
					for _, treeId := range hashGroups[key] {
						fmt.Fprintf(mw, "%d ", treeId)
					}
					fmt.Fprintln(mw)
				}
			} else {
				fmt.Fprintf(mw, "%d: ", key)
				for _, treeId := range hashGroups[key] {
					fmt.Fprintf(mw, "%d ", treeId)
				}
				fmt.Fprintln(mw)
			}
		}
	} else {
		fmt.Printf("hashGroup size: %d \n", len(hashGroups))
	}
}

// func createHashGroupsGoPart2(cTreeIdx <-chan int, centralManagerDone chan<- bool) {
func createHashGroupsGoPart2(cTreeIdx <-chan int) {
	lenSliceTreesG := len(sliceTrees_g)

	//start := time.Now()

	//for treeIdx := range cTreeIdx {
	for i := 0; i < lenSliceTreesG; i++ {
		treeIdx := <-cTreeIdx
		itemTree := sliceTrees_g[treeIdx]
		//itemTree.printTreeContent()
		//fmt.Printf("TreeIdx: %d \n", treeIdx) // debug statement
		hashGroups[itemTree.treeHash] = append(hashGroups[itemTree.treeHash], itemTree.treeId)
	}

	//elapsed := time.Since(start)
	//helper.PrintTime(elapsed, "hashGroupMethodTime")

	//centralManagerDone <- true

	/*
		testSlice := make([]int, 0)
		for i := 0; i < len(sliceTrees_g); i++ {
			testSlice = append(testSlice, <-cTreeIdx)
		}
	*/
}

func createHashGroupsGoPartOpt(cTreeIdx <-chan int, wg2 *sync.WaitGroup, dataWorker int) {
	m := sync.Mutex{}

	wg2.Add(dataWorker)

	for w := 0; w < dataWorker; w++ {
		go func(cTreeIdx <-chan int, m *sync.Mutex, wg2 *sync.WaitGroup) {
			defer wg2.Done()
			for treeIdx := range cTreeIdx {
				itemTree := sliceTrees_g[treeIdx]
				//itemTree.printTreeContent()
				//fmt.Printf("TreeIdx: %d \n", treeIdx) // debug statement
				m.Lock()
				hashGroups[itemTree.treeHash] = append(hashGroups[itemTree.treeHash], itemTree.treeId)
				m.Unlock()
			}
		}(cTreeIdx, &m, wg2)
	}
}

func createHashGroupsGoPartOpt2(cTreeIdx <-chan int, wg2 *sync.WaitGroup, dataWorker int) {
	m := sync.Mutex{}

	wg2.Add(dataWorker)

	for w := 0; w < dataWorker; w++ {
		go func(cTreeIdx <-chan int, m *sync.Mutex, wg2 *sync.WaitGroup, w int) {
			defer wg2.Done()

			for treeIdx := range cTreeIdx {
				itemTree := sliceTrees_g[treeIdx]
				appendTree := true

				//itemTree.printTreeContent()
				//fmt.Printf("TreeIdx: %d \n", treeIdx) // debug statement
				m.Lock()
				// If not found create a new HashGroup.
				hashItem, found := hashGroupsOpt[itemTree.treeHash]
				if !found {
					appendTree = found
					//fmt.Printf("w(%d) - treeId(%d) - add new HashGroup\n", w, treeIdx) // debug statement
					sliceTwins := make([]int, 0)
					sliceTwins = append(sliceTwins, itemTree.treeId)
					hashItem := &HashGroup{
						potTwins: sliceTwins,
						lock:     &sync.Mutex{},
					}
					hashGroupsOpt[itemTree.treeHash] = hashItem
				}
				m.Unlock()

				if appendTree {
					//fmt.Printf("w(%d) - treeId(%d) - add existing tree\n", w, treeIdx) // debug statement
					hashItem.lock.Lock()
					hashItem.potTwins = append(hashItem.potTwins, itemTree.treeId)
					hashItem.lock.Unlock()
				}

			}
		}(cTreeIdx, &m, wg2, w)
	}
}

func convertHashGroupsOptToHashGroups() {
	//fmt.Printf("length: %v \n", len(hashGroupsOpt)) // debug statement
	//fmt.Printf("hashGroupsOpt: %v \n", hashGroupsOpt) // debug statement
	for key := range hashGroupsOpt {
		//fmt.Printf("%v \n", hashGroupsOpt[key].potTwins) // debug statement
		hashGroups[key] = hashGroupsOpt[key].potTwins
	}
}

// ////////////////////// Tree Comparison Groups /////////////////////////
type ComparisonGroup struct {
	groupId string
	same    []string
}

func createComparisonGroups(hashGroupsKey []int) [][]int {
	//func createComparisonGroups() [][]string {
	var comparisonGroups [][]int

	tempPotTwins := make([]int, 0)
	for _, key := range hashGroupsKey {
		tempPotTwins = hashGroups[key]
		tempGroup := make([]int, 0)
		diffGroup := make([]int, 0)
		for len(tempPotTwins) != 0 {
			var itemPotTwin int
			itemPotTwin, tempPotTwins = tempPotTwins[0], tempPotTwins[1:]

			if len(tempGroup) == 0 {
				tempGroup = append(tempGroup, itemPotTwin)
			} else {
				treeA := mapTrees_g[tempGroup[0]]
				treeB := mapTrees_g[itemPotTwin]
				//fmt.Println(treeA.inOrderContent) // debug statement
				if helper.EqualSlices(treeA.inOrderContent, treeB.inOrderContent) {
					tempGroup = append(tempGroup, itemPotTwin)
				} else {
					diffGroup = append(diffGroup, itemPotTwin)
				}
			}

			if len(tempPotTwins) == 0 {
				/*
					if len(tempGroup) > 1 {
						comparisonGroups = append(comparisonGroups, tempGroup)
					}*/
				comparisonGroups = append(comparisonGroups, tempGroup)
				if len(diffGroup) > 0 {
					tempPotTwins = diffGroup
					tempGroup = make([]int, 0)
					diffGroup = make([]int, 0)
				}
			}
		}
	}
	return comparisonGroups
}

func printComparisonGroups(comparisonGroup *[][]int, noLimit bool) {
	groupId := 0
	if len(*comparisonGroup) < 300 || noLimit {
		for i := range *comparisonGroup {
			if len((*comparisonGroup)[i]) > 1 {
				fmt.Fprintf(mw, "group %d: ", groupId)
				for j := range (*comparisonGroup)[i] {
					fmt.Fprintf(mw, "%d ", (*comparisonGroup)[i][j])
				}
				fmt.Fprintln(mw)
				groupId++
			}
		}
	} else {
		for i := range *comparisonGroup {
			if len((*comparisonGroup)[i]) > 1 {
				groupId++
			}
		}
		fmt.Fprintf(mw, "comparisonGroup size: %d \n", groupId)
	}
}

type BufferQueue struct {
	workerCapacity int
	totalWork      int
	workSent       int
	allWorkSent    bool
	queue          []*TreesToCompare
	workerCond     *sync.Cond
	producerCond   *sync.Cond
}

func (q *BufferQueue) pop() *TreesToCompare {
	var item *TreesToCompare
	if q.checkQueueLength() > 0 {
		item, q.queue = q.queue[0], q.queue[1:]
		return item
	}

	// return something if queue is empty
	return nil
}

func (q *BufferQueue) push(treesToCompare *TreesToCompare) bool {
	if q.checkQueueLength() < q.workerCapacity {
		q.queue = append(q.queue, treesToCompare)
		return true
	}

	// return something if queue is full
	return false
}

func (q *BufferQueue) checkQueueLength() int {
	return len(q.queue)
}

type TreesToCompare struct {
	treeA int
	treeB int
}

type EqualTrees struct {
	treeIds []int
	lock    *sync.Mutex
}

/*
type visitedTree struct {
	treeId  int
	visited bool
}
*/

func (et *EqualTrees) addTree(tree int) {
	et.treeIds = append(et.treeIds, tree)
}

// func createComparisonGroupsGo(compWorkers int, wg *sync.WaitGroup) [][]string {
func createComparisonGroupsGo(compWorkers int, wg *sync.WaitGroup) (retEQTG *[]*EqualTrees) {
	//var comparisonGroups [][]string
	lenSliceTreesG := len(sliceTrees_g)
	qLock := &sync.Mutex{}

	// Create the queue
	queue := &BufferQueue{
		workerCapacity: compWorkers,
		totalWork:      lenSliceTreesG * lenSliceTreesG,
		workSent:       0,
		allWorkSent:    false,
		queue:          make([]*TreesToCompare, 0),
		workerCond:     sync.NewCond(qLock),
		producerCond:   sync.NewCond(qLock),
	}

	// Create a sparse matrix
	equalTreesGroup := make([]*EqualTrees, lenSliceTreesG)
	for key := range equalTreesGroup {
		tempTreeIds := make([]int, 0)
		tempTreeIds = append(tempTreeIds, key)
		equalTreesGroup[key] = &EqualTrees{
			treeIds: tempTreeIds,
			lock:    &sync.Mutex{},
		}
	}

	//printEqualTrees(&equalTreesGroup) // debug statement

	/*
		// Create sparse adjacency matrix with hashgroups to define work.
		adjPairsToVisit := make(map[string]*TreesToCompare)

		mapId := 0
		for i := range hashGroups {
			for j := range hashGroups[i] {
				for k := range hashGroups[i] {
					if hashGroups[i][j] != hashGroups[i][k] {
						mapKey := strconv.Itoa(mapId) + "-" + strconv.Itoa(hashGroups[i][j]) + strconv.Itoa(hashGroups[i][k])
						adjPairsToVisit[mapKey] = &TreesToCompare{
							treeA: hashGroups[i][j],
							treeB: hashGroups[i][k],
						}
						mapId++
					}
				}
			}
		}
	*/

	//fmt.Println(adjPairsToVisit) // debug statement

	// start workers
	for w := 0; w < compWorkers; w++ {
		wg.Add(1)
		//go comparisonWorker(queue, &sliceTrees_g, wg, &adjMatrix)
		go comparisonWorker(queue, &sliceTrees_g, wg, &equalTreesGroup)
	}

	/*
		for _, work := range adjPairsToVisit {
			// pass tree pair
			queue.producerCond.L.Lock()
			for queue.checkQueueLength() == queue.workerCapacity {
				//fmt.Printf("I'm waiting at %d \n", (i*lenSliceTreesG)+j)
				queue.producerCond.Wait()
			}
			queue.push(work)

			queue.workSent++
			queue.workerCond.Signal()
			queue.producerCond.L.Unlock()
		}
	*/

	for i := range hashGroups {
		for j := range hashGroups[i] {
			for k := range hashGroups[i] {
				if hashGroups[i][j] != hashGroups[i][k] {
					work := &TreesToCompare{
						treeA: hashGroups[i][j],
						treeB: hashGroups[i][k],
					}
					// pass tree pair
					queue.producerCond.L.Lock()
					for queue.checkQueueLength() == queue.workerCapacity {
						//fmt.Printf("I'm waiting at %d \n", (i*lenSliceTreesG)+j)
						queue.producerCond.Wait()
					}
					queue.push(work)

					queue.workSent++
					queue.workerCond.Signal()
					queue.producerCond.L.Unlock()
				}
			}
		}
	}

	queue.producerCond.L.Lock()
	queue.allWorkSent = true
	queue.producerCond.L.Unlock()
	//fmt.Println("I'm done.  Wake up all sleepy routines to go away!") // debug statement
	queue.workerCond.Broadcast()

	wg.Wait()
	//fmt.Println(adjMatrix) // debug statement for adjacency matrix
	//printEqualTrees(&equalTreesGroup) // debug statement

	retEQTG = &equalTreesGroup

	return retEQTG
}

// func comparisonWorker(q *BufferQueue, sliceTrees *[]*Tree, wg *sync.WaitGroup, adjMatrix *[][]bool) {
func comparisonWorker(q *BufferQueue, sliceTrees *[]*Tree, wg *sync.WaitGroup, equalTreesGroup *[]*EqualTrees) {
	for {
		q.workerCond.L.Lock()
		//fmt.Println(q.queue) // debug statement
		for q.checkQueueLength() == 0 && q.allWorkSent == false {
			q.workerCond.Wait()
		}

		item := q.pop()

		if item != nil {
			// use adjacency matrix
			//(*adjMatrix)[item.treeA][item.treeB] = helper.EqualSlices((*sliceTrees)[item.treeA].inOrderContent, (*sliceTrees)[item.treeB].inOrderContent)

			// use a sparse matrix-like structure

			if helper.EqualSlices((*sliceTrees)[item.treeA].inOrderContent, (*sliceTrees)[item.treeB].inOrderContent) {
				(*equalTreesGroup)[item.treeA].lock.Lock()
				(*equalTreesGroup)[item.treeA].treeIds = append((*equalTreesGroup)[item.treeA].treeIds, item.treeB)
				(*equalTreesGroup)[item.treeA].lock.Unlock()
			}
		}

		q.producerCond.Signal()

		if q.allWorkSent == true {
			//fmt.Printf("workSent is %d, I'm leaving!\n", q.workSent) // debug statement
			q.workerCond.L.Unlock()
			break
		}
		q.workerCond.L.Unlock()
	}
	wg.Done()
}

func printEqualTrees(equalTreesGroup *[]*EqualTrees) {
	fmt.Println("Equal Trees Sparse Matrix")
	for key := range *equalTreesGroup {
		fmt.Printf("[%d] %v \n", key, (*equalTreesGroup)[key].treeIds)
	}
}

func convertToComparisonGroups(equalTreesGroup *[]*EqualTrees) *[][]int {
	var comparisonGroups [][]int
	sliceTreesG := len(sliceTrees_g)
	visitedTrees := make([]bool, sliceTreesG)
	sortedKeys := make([]int, 0)

	for key := range *equalTreesGroup {
		sortedKeys = append(sortedKeys, key)
		//fmt.Printf("key: %d %v\n", key, (*equalTreesGroup)[key].treeIds) // debug statement
	}
	sort.Ints(sortedKeys)

	for key := range sortedKeys {
		//fmt.Printf("key: %d\n", key) // debug statemet
		if !visitedTrees[key] {
			for i := range (*equalTreesGroup)[key].treeIds {
				visitedTrees[(*equalTreesGroup)[key].treeIds[i]] = true
			}
			comparisonGroups = append(comparisonGroups, (*equalTreesGroup)[key].treeIds)
		}
	}

	return &comparisonGroups
}

func part3FirstImplementation(hashGroupsKey []int, wg *sync.WaitGroup) [][]bool {
	lenSliceTreesG := len(sliceTrees_g)
	m := &sync.Mutex{}

	// Adjacency matrix setup.
	adjMatrix := make([][]bool, lenSliceTreesG)
	for i := range adjMatrix {
		adjMatrix[i] = make([]bool, lenSliceTreesG)
	}

	for _, key := range hashGroupsKey {
		if len(hashGroups[key]) == 1 {
			adjMatrix[hashGroups[key][0]][hashGroups[key][0]] = true
		} else {
			for i := range hashGroups[key] {
				for j := range hashGroups[key] {
					wg.Add(1)
					go func(m *sync.Mutex, i int, j int, key int) {
						defer wg.Done()
						//fmt.Printf("i(%d) = %d, j(%d) = %d; ", i, hashGroups[key][i], j, hashGroups[key][j]) //debug statement
						//m.Lock()
						adjMatrix[hashGroups[key][i]][hashGroups[key][j]] = helper.EqualSlices(sliceTrees_g[hashGroups[key][i]].inOrderContent, sliceTrees_g[hashGroups[key][j]].inOrderContent)
						//m.Unlock()
					}(m, i, j, key)
				}
			}
		}
	}

	wg.Wait()

	return adjMatrix
}

func printPart3FirstImplementation(input [][]bool, mw *io.Writer, noLimit bool) {
	fmt.Fprintln(*mw, "Adjacency Matrix:")
	if len(input) < 300 || noLimit {
		for i := range input {
			for j := range input[i] {
				fmt.Fprintf(*mw, "%v ", input[i][j])
			}
			fmt.Fprintln(*mw)
		}
	} else {
		fmt.Fprintln(*mw, "Matrix too large to print.")
	}
}
