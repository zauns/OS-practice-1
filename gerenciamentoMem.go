package main

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

// ─────────────────────────────────────────────
// Shared helpers
// ─────────────────────────────────────────────

var scanner = bufio.NewScanner(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func readInt(prompt string) int {
	for {
		s := readLine(prompt)
		v, err := strconv.Atoi(s)
		if err == nil && v >= 0 {
			return v
		}
		fmt.Println("  ✗ Por favor, insira um número inteiro positivo.")
	}
}

func readIntMin(prompt string, min int) int {
	for {
		v := readInt(prompt)
		if v >= min {
			return v
		}
		fmt.Printf("  ✗ O valor mínimo é %d.\n", min)
	}
}

func sep(char string, n int) string { return strings.Repeat(char, n) }

func centerPad(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := width - len(s)
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func rpad(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// ─────────────────────────────────────────────
// PART A — Variable Partition Allocation
// ─────────────────────────────────────────────

type ProcessA struct {
	ID         int    // unique process identifier
	Name       string // process name
	Size       int    // size in memory units
	InMem      bool   // whether process is currently in physical memory
	SwappedOut bool   // whether process was swapped to secondary storage
}

type Partition struct {
	Start  int // starting address in memory
	Size   int // size of partition
	ProcID int // process ID occupying this partition (-1 if free)
}

type PartitionState struct {
	MemSize    int          // total physical memory size
	Unit       string       // unit of measurement (MB, KB, etc.)
	Algorithm  string       // allocation algorithm ("first-fit" | "best-fit" | "worst-fit")
	SwapPolicy string       // swap-out policy ("random" | "fifo" | "off")
	Processes  []*ProcessA  // list of all processes
	Partitions []Partition  // list of memory partitions
	NextProcID int          // counter for next process ID
	Allocated  bool         // whether allocation has been run
}

func newPartitionState() *PartitionState {
	s := &PartitionState{
		MemSize:   512,
		Unit:      "MB",
		Algorithm: "first-fit",
		SwapPolicy: "random",
		NextProcID: 1,
	}
	// Seed with default processes to demonstrate allocation, compaction, and swapping.
	for _, p := range []struct {
		name string
		size int
	}{
		{"P1", 120},
		{"P2", 80},
		{"P3", 200},
		{"P4", 100},
		{"P5", 120},
		{"P6", 92},
		{"P7", 20},
		{"P8", 40},
		{"P9", 60},
		{"P10", 72},
		{"P11", 32},
		{"P12", 140},
	} {
		s.addProcess(p.name, p.size)
	}
	return s
}

func (s *PartitionState) reset() {
	s.Processes = nil
	s.Partitions = nil
	s.NextProcID = 1
	s.Allocated = false
}

func (s *PartitionState) addProcess(name string, size int) {
	s.Processes = append(s.Processes, &ProcessA{
		ID:   s.NextProcID,
		Name: name,
		Size: size,
	})
	s.NextProcID++
	fmt.Printf("  ✓ Processo '%s' (ID=%d, %d %s) adicionado.\n", name, s.NextProcID-1, size, s.Unit)
}

func (s *PartitionState) removeProcess(id int) {
	for i, p := range s.Processes {
		if p.ID == id {
			s.Processes = append(s.Processes[:i], s.Processes[i+1:]...)
			fmt.Printf("  ✓ Processo ID=%d removido.\n", id)
			return
		}
	}
	fmt.Println("  ✗ Processo não encontrado.")
}

// tryAlloc attempts to allocate proc using current algorithm.
// Returns the index of the chosen partition, or -1.
func (s *PartitionState) tryAlloc(proc *ProcessA) int {
	best := -1
	switch s.Algorithm {
	case "first-fit":
		// Find the first free partition large enough to fit the process
		for i, p := range s.Partitions {
			if p.ProcID == -1 && p.Size >= proc.Size {
				best = i
				break
			}
		}
	case "best-fit":
		// Find the smallest free partition that still fits the process (minimizes waste)
		bestSz := math.MaxInt64
		for i, p := range s.Partitions {
			if p.ProcID == -1 && p.Size >= proc.Size && p.Size < bestSz {
				best = i
				bestSz = p.Size
			}
		}
	case "worst-fit":
		// Find the largest free partition to fit the process (maximizes remaining space)
		worstSz := -1
		for i, p := range s.Partitions {
			if p.ProcID == -1 && p.Size >= proc.Size && p.Size > worstSz {
				best = i
				worstSz = p.Size
			}
		}
	}
	return best
}

// allocAt places proc into partition at index idx.
func (s *PartitionState) allocAt(idx int, proc *ProcessA) {
	p := s.Partitions[idx]
	remainder := p.Size - proc.Size
	s.Partitions[idx] = Partition{Start: p.Start, Size: proc.Size, ProcID: proc.ID}
	proc.InMem = true
	if remainder > 0 {
		// insert free block after
		newParts := make([]Partition, 0, len(s.Partitions)+1)
		newParts = append(newParts, s.Partitions[:idx+1]...)
		newParts = append(newParts, Partition{Start: p.Start + proc.Size, Size: remainder, ProcID: -1})
		newParts = append(newParts, s.Partitions[idx+1:]...)
		s.Partitions = newParts
	}
}

// compact merges free partitions and shifts allocated ones to low addresses.
func (s *PartitionState) compact() {
	fmt.Println("  ↯ Realizando compactação de memória...")
	var used []Partition
	for _, p := range s.Partitions {
		if p.ProcID != -1 {
			used = append(used, p)
		}
	}
	cursor := 0
	s.Partitions = nil
	for _, p := range used {
		s.Partitions = append(s.Partitions, Partition{Start: cursor, Size: p.Size, ProcID: p.ProcID})
		cursor += p.Size
	}
	free := s.MemSize - cursor
	if free > 0 {
		s.Partitions = append(s.Partitions, Partition{Start: cursor, Size: free, ProcID: -1})
	}
}

// swapOutRandom removes a random in-memory process.
func (s *PartitionState) swapOutRandom() bool {
	var candidates []*ProcessA
	for _, p := range s.Processes {
		if p.InMem {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return false
	}
	victim := candidates[rand.Intn(len(candidates))]
	fmt.Printf("  ↯ Swap-out (aleatório): processo '%s' (ID=%d) enviado para memória secundária.\n", victim.Name, victim.ID)
	victim.InMem = false
	victim.SwappedOut = true
	// free its partition
	for i, p := range s.Partitions {
		if p.ProcID == victim.ID {
			s.Partitions[i].ProcID = -1
			break
		}
	}
	// merge adjacent free blocks
	s.mergeFree()
	return true
}

// swapOutFIFO removes the oldest in-memory process (by creation order).
func (s *PartitionState) swapOutFIFO() bool {
	var victim *ProcessA
	for _, p := range s.Processes {
		if p.InMem {
			victim = p
			break
		}
	}
	if victim == nil {
		return false
	}
	fmt.Printf("  ↯ Swap-out (FIFO): processo '%s' (ID=%d) enviado para memória secundária.\n", victim.Name, victim.ID)
	victim.InMem = false
	victim.SwappedOut = true
	// free its partition
	for i, p := range s.Partitions {
		if p.ProcID == victim.ID {
			s.Partitions[i].ProcID = -1
			break
		}
	}
	// merge adjacent free blocks
	s.mergeFree()
	return true
}

func (s *PartitionState) mergeFree() {
	merged := []Partition{s.Partitions[0]}
	for i := 1; i < len(s.Partitions); i++ {
		last := &merged[len(merged)-1]
		cur := s.Partitions[i]
		if last.ProcID == -1 && cur.ProcID == -1 {
			last.Size += cur.Size
		} else {
			merged = append(merged, cur)
		}
	}
	s.Partitions = merged
}

func (s *PartitionState) runAllocation() {
	if len(s.Processes) == 0 {
		fmt.Println("  ✗ Nenhum processo definido.")
		return
	}
	// init memory as single free block
	s.Partitions = []Partition{{Start: 0, Size: s.MemSize, ProcID: -1}}
	// reset process state
	for _, p := range s.Processes {
		p.InMem = false
		p.SwappedOut = false
	}
	s.Allocated = true

	fmt.Printf("\n  Alocando %d processo(s) em ordem FIFO com algoritmo '%s' (swap-out: %s)...\n\n", len(s.Processes), s.Algorithm, s.SwapPolicy)

	for _, proc := range s.Processes {
		if proc.Size > s.MemSize {
			fmt.Printf("  ✗ Processo '%s' (%d %s) maior que a memória total. Ignorado.\n", proc.Name, proc.Size, s.Unit)
			continue
		}
		fmt.Printf("  → Alocando '%s' (%d %s)...\n", proc.Name, proc.Size, s.Unit)
		idx := s.tryAlloc(proc)
		if idx >= 0 {
			chosen := s.Partitions[idx].Size
			leftover := chosen - proc.Size
			s.allocAt(idx, proc)
			fmt.Printf("    ✓ Alocado na partição iniciando em %d %s.\n", s.Partitions[idx].Start, s.Unit)
			if s.Algorithm == "best-fit" {
				fmt.Printf("    ↳ Best-fit escolheu bloco de %d %s (sobra %d %s).\n", chosen, s.Unit, leftover, s.Unit)
			}
			continue
		}
		// try compaction
		s.compact()
		idx = s.tryAlloc(proc)
		if idx >= 0 {
			chosen := s.Partitions[idx].Size
			leftover := chosen - proc.Size
			s.allocAt(idx, proc)
			fmt.Printf("    ✓ Alocado após compactação na partição iniciando em %d %s.\n", s.Partitions[idx].Start, s.Unit)
			if s.Algorithm == "best-fit" {
				fmt.Printf("    ↳ Best-fit escolheu bloco de %d %s (sobra %d %s).\n", chosen, s.Unit, leftover, s.Unit)
			}
			continue
		}
		// swap out (optional)
		if s.SwapPolicy == "off" {
			fmt.Printf("    ✗ Swap-out desativado; não foi possível alocar '%s'.\n", proc.Name)
			continue
		}
		swapped := false
		switch s.SwapPolicy {
		case "fifo":
			swapped = s.swapOutFIFO()
		default:
			swapped = s.swapOutRandom()
		}
		if !swapped {
			fmt.Printf("    ✗ Não há processos para swap-out; '%s' não foi alocado.\n", proc.Name)
			continue
		}
		s.compact()
		idx = s.tryAlloc(proc)
		if idx >= 0 {
			s.allocAt(idx, proc)
			fmt.Printf("    ✓ Alocado após swap-out na partição iniciando em %d %s.\n", s.Partitions[idx].Start, s.Unit)
		} else {
			fmt.Printf("    ✗ Não foi possível alocar '%s' mesmo após compactação e swap-out.\n", proc.Name)
		}
	}
	fmt.Println()
	s.showMemoryMap()
}

func (s *PartitionState) showMemoryMap() {
	if !s.Allocated {
		fmt.Println("  (Execute a alocação primeiro — opção 5)")
		return
	}
	fmt.Printf("  Política de swap-out: %s\n", s.SwapPolicy)
	const w1, w2, w3, w4 = 8, 14, 8, 8
	line := fmt.Sprintf("+%s+%s+%s+%s+", sep("-", w1+2), sep("-", w2+2), sep("-", w3+2), sep("-", w4+2))
	fmt.Println(line)
	fmt.Printf("| %s | %s | %s | %s |\n",
		centerPad("Início", w1), centerPad("Processo", w2),
		centerPad("Tam.", w3), centerPad("Status", w4))
	fmt.Println(line)
	for _, p := range s.Partitions {
		name := "[livre]"
		status := "LIVRE"
		if p.ProcID != -1 {
			status = "USADO"
			for _, proc := range s.Processes {
				if proc.ID == p.ProcID {
					name = proc.Name
					break
				}
			}
		}
		fmt.Printf("| %s | %s | %s | %s |\n",
			rpad(fmt.Sprintf("%d %s", p.Start, s.Unit), w1),
			rpad(name, w2),
			rpad(fmt.Sprintf("%d %s", p.Size, s.Unit), w3),
			centerPad(status, w4))
	}
	fmt.Println(line)

	// swapped
	var swapped []string
	for _, p := range s.Processes {
		if p.SwappedOut {
			swapped = append(swapped, fmt.Sprintf("%s(ID=%d)", p.Name, p.ID))
		}
	}
	if len(swapped) > 0 {
		fmt.Printf("  Swapped out (memória secundária): %s\n", strings.Join(swapped, ", "))
	}
}

func (s *PartitionState) showFragmentation() {
	if !s.Allocated {
		fmt.Println("  (Execute a alocação primeiro — opção 5)")
		return
	}
	// External fragmentation = free memory that is not contiguous
	var freeBlocks []int
	totalFree := 0
	fmt.Printf("  Política de swap-out: %s\n", s.SwapPolicy)
	for _, p := range s.Partitions {
		if p.ProcID == -1 {
			freeBlocks = append(freeBlocks, p.Size)
			totalFree += p.Size
		}
	}
	var extFrag int
	if len(freeBlocks) > 1 {
		// all free memory except the largest block
		maxFree := 0
		for _, f := range freeBlocks {
			if f > maxFree {
				maxFree = f
			}
		}
		extFrag = totalFree - maxFree
	}
	// Internal fragmentation: for variable partitions = 0 by definition
	fmt.Println("\n  ─── Relatório de Fragmentação ───")
	fmt.Printf("  Memória total        : %d %s\n", s.MemSize, s.Unit)
	fmt.Printf("  Memória livre total  : %d %s (%d bloco(s))\n", totalFree, s.Unit, len(freeBlocks))
	fmt.Printf("  Fragmentação interna : 0 %s (partições variáveis — sem desperdício por partição)\n", s.Unit)
	fmt.Printf("  Fragmentação externa : %d %s", extFrag, s.Unit)
	if len(freeBlocks) > 0 {
		fmt.Printf(" (%d bloco(s) livre(s): ", len(freeBlocks))
		for i, f := range freeBlocks {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%d %s", f, s.Unit)
		}
		fmt.Print(")")
	}
	fmt.Println()
}

func menuPartition(s *PartitionState) {
	for {
		fmt.Printf("\n╔══════════════════════════════════════╗\n")
		fmt.Printf("║      ALOCAÇÃO POR PARTIÇÕES          ║\n")
		fmt.Printf("╚══════════════════════════════════════╝\n")
		fmt.Printf("  Config: Memória=%d%s  Algoritmo=%s  Swap=%s\n", s.MemSize, s.Unit, s.Algorithm, s.SwapPolicy)
		fmt.Printf("  Processos definidos: %d\n", len(s.Processes))
		fmt.Println()
		fmt.Println("  1. Definir tamanho da memória física")
		fmt.Println("  2. Definir algoritmo de alocação")
		fmt.Println("  3. Definir política de swap-out")
		fmt.Println("  4. Adicionar processo")
		fmt.Println("  5. Remover processo")
		fmt.Println("  6. Executar alocação")
		fmt.Println("  7. Mostrar mapa de memória")
		fmt.Println("  8. Mostrar relatório de fragmentação")
		fmt.Println("  9. Listar processos")
		fmt.Println("  10. Reiniciar")
		fmt.Println("  0. Voltar")
		fmt.Println()

		choice := readLine("  Opção: ")
		switch choice {
		case "1":
			s.MemSize = readIntMin("  Tamanho da memória (inteiro): ", 1)
			s.Unit = readLine("  Unidade (MB/KB/etc.): ")
			if s.Unit == "" {
				s.Unit = "MB"
			}
			s.Allocated = false
		case "2":
			fmt.Println("  1. first-fit")
			fmt.Println("  2. best-fit")
			fmt.Println("  3. worst-fit")
			algoChoice := readLine("  Escolha: ")
			switch algoChoice {
			case "1":
				s.Algorithm = "first-fit"
			case "2":
				s.Algorithm = "best-fit"
			case "3":
				s.Algorithm = "worst-fit"
			default:
				fmt.Println("  ✗ Opção inválida.")
			}
		case "3":
			fmt.Println("  1. Aleatório")
			fmt.Println("  2. FIFO (ordem de criação)")
			fmt.Println("  3. Desativar swap-out")
			sp := readLine("  Escolha: ")
			switch sp {
			case "1":
				s.SwapPolicy = "random"
			case "2":
				s.SwapPolicy = "fifo"
			case "3":
				s.SwapPolicy = "off"
			default:
				fmt.Println("  ✗ Opção inválida.")
			}
		case "4":
			name := readLine("  Nome do processo: ")
			if name == "" {
				fmt.Println("  ✗ Nome não pode ser vazio.")
				continue
			}
			size := readIntMin("  Tamanho ("+s.Unit+"): ", 1)
			s.addProcess(name, size)
		case "5":
			s.listProcesses()
			id := readInt("  ID do processo a remover: ")
			s.removeProcess(id)
		case "6":
			s.runAllocation()
		case "7":
			s.showMemoryMap()
		case "8":
			s.showFragmentation()
		case "9":
			s.listProcesses()
		case "10":
			s.reset()
			fmt.Println("  ✓ Estado reiniciado.")
		case "0":
			return
		default:
			fmt.Println("  ✗ Opção inválida.")
		}
	}
}

func (s *PartitionState) listProcesses() {
	if len(s.Processes) == 0 {
		fmt.Println("  (nenhum processo)")
		return
	}
	fmt.Printf("  Política de swap-out: %s\n", s.SwapPolicy)
	fmt.Printf("  %-4s %-14s %-8s %-10s %-10s\n", "ID", "Nome", "Tam.", "Na Mem.", "Swapped")
	fmt.Printf("  %s\n", sep("-", 50))
	for _, p := range s.Processes {
		inMem := "não"
		if p.InMem {
			inMem = "sim"
		}
		swapped := "não"
		if p.SwappedOut {
			swapped = "sim"
		}
		fmt.Printf("  %-4d %-14s %-8d %-10s %-10s\n", p.ID, p.Name, p.Size, inMem, swapped)
	}
}

// ─────────────────────────────────────────────
// PART B — Paging
// ─────────────────────────────────────────────

type ProcessB struct {
	ID    int
	Name  string
	Size  int // in unit
	Pages int // ceil(Size/PageSize)
}

type FrameB struct {
	ProcID  int // -1 if free
	PageNum int // local page number within the process
}

type PageEntry struct {
	FrameNum     int  // -1 if not in memory
	Valid        bool
	RefBit       bool
	LastUsed     int64 // for LRU
	LoadedAt     int64 // for FIFO (logical clock when loaded)
}

type PagingState struct {
	PhysMem    int
	VirtMem    int
	PageSize   int
	Unit       string
	Algorithm  string // "fifo" | "lru" | "clock"
	ClockK     int    // reset interval for Clock bits
	Processes  []*ProcessB
	Frames     []FrameB
	PageTables map[int][]PageEntry // procID -> []PageEntry indexed by page number
	NextProcID int
	Simulated  bool
	// Stats
	PageFaults map[string]int // algorithm -> count
}

func newPagingState() *PagingState {
	s := &PagingState{
		PhysMem:    256,
		VirtMem:    512,
		PageSize:   32,
		Unit:       "MB",
		Algorithm:  "fifo",
		ClockK:     0, // 0 = use numFrames as default
		NextProcID: 1,
		PageFaults: make(map[string]int),
	}
	// Seed with default processes to demonstrate page replacement algorithms.
	for _, p := range []struct {
		name string
		size int
	}{
		{"A", 90},
		{"B", 120},
		{"C", 64},
		{"D", 140},
		{"E", 70},
	} {
		s.addProcess(p.name, p.size)
	}
	return s
}

func (s *PagingState) numFrames() int { return s.PhysMem / s.PageSize }
func (s *PagingState) numVirtPages() int { return s.VirtMem / s.PageSize }

func (s *PagingState) resetSim() {
	s.Frames = make([]FrameB, s.numFrames())
	for i := range s.Frames {
		s.Frames[i] = FrameB{ProcID: -1, PageNum: -1}
	}
	s.PageTables = make(map[int][]PageEntry)
	for _, p := range s.Processes {
		entries := make([]PageEntry, p.Pages)
		for i := range entries {
			entries[i] = FrameNum(-1)
		}
		s.PageTables[p.ID] = entries
	}
	s.PageFaults = make(map[string]int)
	s.Simulated = false
}

// FrameNum returns a PageEntry with FrameNum set (helper for clarity)
func FrameNum(n int) PageEntry {
	return PageEntry{FrameNum: n, Valid: false}
}

func (s *PagingState) validate() error {
	if s.VirtMem <= s.PhysMem {
		return fmt.Errorf("memória virtual (%d) deve ser maior que a física (%d)", s.VirtMem, s.PhysMem)
	}
	if s.PhysMem%s.PageSize != 0 {
		return fmt.Errorf("tamanho de página (%d) não divide a memória física (%d) igualmente", s.PageSize, s.PhysMem)
	}
	if s.VirtMem%s.PageSize != 0 {
		return fmt.Errorf("tamanho de página (%d) não divide a memória virtual (%d) igualmente", s.PageSize, s.VirtMem)
	}
	return nil
}

func (s *PagingState) addProcess(name string, size int) {
	pages := int(math.Ceil(float64(size) / float64(s.PageSize)))
	maxVirtPages := s.numVirtPages()
	if pages > maxVirtPages {
		fmt.Printf("  ✗ Processo '%s' requer %d páginas mas a memória virtual só suporta %d.\n", name, pages, maxVirtPages)
		return
	}
	s.Processes = append(s.Processes, &ProcessB{
		ID:    s.NextProcID,
		Name:  name,
		Size:  size,
		Pages: pages,
	})
	fmt.Printf("  ✓ Processo '%s' (ID=%d, %d %s, %d páginas) adicionado.\n", name, s.NextProcID, size, s.Unit, pages)
	s.NextProcID++
}

// ── Simulation core ──────────────────────────

type simState struct {
	frames    []FrameB
	pageTbl   map[int][]PageEntry
	clock     int   // clock hand for Clock algorithm
	logicalTS int64 // for LRU
	fifoQueue []struct{ proc, page int } // for FIFO eviction
}

func newSimState(nFrames int, procs []*ProcessB) *simState {
	frames := make([]FrameB, nFrames)
	for i := range frames {
		frames[i] = FrameB{ProcID: -1, PageNum: -1}
	}
	pt := make(map[int][]PageEntry)
	for _, p := range procs {
		entries := make([]PageEntry, p.Pages)
		for i := range entries {
			entries[i] = PageEntry{FrameNum: -1}
		}
		pt[p.ID] = entries
	}
	return &simState{frames: frames, pageTbl: pt}
}

func (ss *simState) freeFrame() int {
	for i, f := range ss.frames {
		if f.ProcID == -1 {
			return i
		}
	}
	return -1
}

// evict returns the frame index to evict, using given algorithm.
func (ss *simState) evict(algo string, k int) int {
	switch algo {
	case "fifo":
		if len(ss.fifoQueue) == 0 {
			return 0
		}
		victim := ss.fifoQueue[0]
		ss.fifoQueue = ss.fifoQueue[1:]
		// find frame for this victim
		for i, f := range ss.frames {
			if f.ProcID == victim.proc && f.PageNum == victim.page {
				return i
			}
		}
		return 0

	case "lru":
		minTS := int64(math.MaxInt64)
		idx := 0
		for i, f := range ss.frames {
			if f.ProcID == -1 {
				continue
			}
			e := ss.pageTbl[f.ProcID][f.PageNum]
			if e.LastUsed < minTS {
				minTS = e.LastUsed
				idx = i
			}
		}
		return idx

	case "clock":
		nf := len(ss.frames)
		for {
			f := ss.frames[ss.clock]
			if f.ProcID == -1 {
				chosen := ss.clock
				ss.clock = (ss.clock + 1) % nf
				return chosen
			}
			e := ss.pageTbl[f.ProcID][f.PageNum]
			if !e.RefBit {
				chosen := ss.clock
				ss.clock = (ss.clock + 1) % nf
				return chosen
			}
			// give second chance
			ss.pageTbl[f.ProcID][f.PageNum].RefBit = false
			ss.clock = (ss.clock + 1) % nf
		}
	}
	return 0
}

// loadPage loads procID:pageNum into the given frame, evicting if needed.
func (ss *simState) loadPage(procID, pageNum, frameIdx int, algo string, loadClock int64) {
	// invalidate old occupant
	old := ss.frames[frameIdx]
	if old.ProcID != -1 {
		ss.pageTbl[old.ProcID][old.PageNum].Valid = false
		ss.pageTbl[old.ProcID][old.PageNum].FrameNum = -1
	}
	ss.frames[frameIdx] = FrameB{ProcID: procID, PageNum: pageNum}
	ss.pageTbl[procID][pageNum] = PageEntry{
		FrameNum:  frameIdx,
		Valid:     true,
		RefBit:    true,
		LastUsed:  loadClock,
		LoadedAt:  loadClock,
	}
	if algo == "fifo" {
		ss.fifoQueue = append(ss.fifoQueue, struct{ proc, page int }{procID, pageNum})
	}
}

// reference handles a single page reference. Returns true if page fault.
func (ss *simState) reference(procID, pageNum int, algo string, k int) bool {
	ss.logicalTS++
	entry := ss.pageTbl[procID][pageNum]
	if entry.Valid {
		// page hit
		ss.pageTbl[procID][pageNum].LastUsed = ss.logicalTS
		ss.pageTbl[procID][pageNum].RefBit = true
		// update fifo queue position not needed for FIFO (only on load)
		return false
	}
	// page fault
	frameIdx := ss.freeFrame()
	if frameIdx == -1 {
		frameIdx = ss.evict(algo, k)
	}
	ss.loadPage(procID, pageNum, frameIdx, algo, ss.logicalTS)
	return true
}

// resetRefBits resets all reference bits (Clock periodic reset)
func (ss *simState) resetRefBits() {
	for _, f := range ss.frames {
		if f.ProcID != -1 {
			ss.pageTbl[f.ProcID][f.PageNum].RefBit = false
		}
	}
}

// buildRefString builds a reference string of length n.
// mode: "fifo" = round-robin sequential; "random" = random
func buildRefString(procs []*ProcessB, n int, mode string) [][2]int {
	var refs [][2]int
	if mode == "fifo" {
		// round-robin each process's pages
		i := 0
		pageIdx := make(map[int]int)
		for len(refs) < n {
			p := procs[i%len(procs)]
			pg := pageIdx[p.ID] % p.Pages
			refs = append(refs, [2]int{p.ID, pg})
			pageIdx[p.ID]++
			i++
		}
	} else {
		// random
		for j := 0; j < n; j++ {
			p := procs[rand.Intn(len(procs))]
			pg := rand.Intn(p.Pages)
			refs = append(refs, [2]int{p.ID, pg})
		}
	}
	return refs
}

func (s *PagingState) runSimulation() {
	if err := s.validate(); err != nil {
		fmt.Printf("  ✗ Configuração inválida: %s\n", err)
		return
	}
	if len(s.Processes) == 0 {
		fmt.Println("  ✗ Nenhum processo definido.")
		return
	}

	// ask reference string settings
	fmt.Println("\n  ─── Configuração da String de Referência ───")
	fmt.Println("  1. FIFO (round-robin sequencial das páginas)")
	fmt.Println("  2. Aleatório")
	refMode := readLine("  Modo: ")
	if refMode != "1" && refMode != "2" {
		fmt.Println("  ✗ Opção inválida.")
		return
	}
	modeLabel := "fifo"
	if refMode == "2" {
		modeLabel = "random"
	}
	n := readIntMin("  Número de referências a simular: ", 1)

	refs := buildRefString(s.Processes, n, modeLabel)

	// determine which algorithms to compare
	algos := []string{s.Algorithm}
	// always compare all three for the stats table
	algos = []string{"fifo", "lru", "clock"}

	effectiveK := s.ClockK
	if effectiveK == 0 {
		effectiveK = s.numFrames()
	}

	fmt.Printf("\n  Simulando %d referências com %d quadros (%d %s/quadro)...\n",
		n, s.numFrames(), s.PageSize, s.Unit)
	fmt.Printf("  Algoritmos comparados: FIFO, LRU, Clock (K=%d)\n\n", effectiveK)

	s.PageFaults = make(map[string]int)
	var lastSS *simState

	for _, algo := range algos {
		ss := newSimState(s.numFrames(), s.Processes)
		faults := 0
		for idx, ref := range refs {
			fault := ss.reference(ref[0], ref[1], algo, effectiveK)
			if fault {
				faults++
			}
			// Clock: reset ref bits every K references
			if algo == "clock" && (idx+1)%effectiveK == 0 {
				ss.resetRefBits()
			}
		}
		s.PageFaults[algo] = faults
		if algo == s.Algorithm {
			lastSS = ss
		}
	}
	// keep last sim state for display
	if lastSS != nil {
		s.Frames = lastSS.frames
		s.PageTables = lastSS.pageTbl
	}
	s.Simulated = true

	// Show summary
	fmt.Println("  ─── Resultado da Simulação ───")
	s.showPageFaultStats()
}

func (s *PagingState) showFrameTable() {
	if !s.Simulated {
		fmt.Println("  (Execute a simulação primeiro — opção 8)")
		return
	}
	const w1, w2, w3, w4 = 7, 14, 8, 6
	line := fmt.Sprintf("+%s+%s+%s+%s+", sep("-", w1+2), sep("-", w2+2), sep("-", w3+2), sep("-", w4+2))
	fmt.Println(line)
	fmt.Printf("| %s | %s | %s | %s |\n",
		centerPad("Quadro", w1), centerPad("Processo", w2),
		centerPad("Pág.", w3), centerPad("Válid.", w4))
	fmt.Println(line)
	for i, f := range s.Frames {
		name := "[livre]"
		pg := "-"
		valid := "não"
		if f.ProcID != -1 {
			valid = "sim"
			pg = fmt.Sprintf("%d", f.PageNum)
			for _, p := range s.Processes {
				if p.ID == f.ProcID {
					name = p.Name
					break
				}
			}
		}
		fmt.Printf("| %s | %s | %s | %s |\n",
			rpad(fmt.Sprintf("%d", i), w1),
			rpad(name, w2),
			centerPad(pg, w3),
			centerPad(valid, w4))
	}
	fmt.Println(line)
}

func (s *PagingState) showPageTables() {
	if !s.Simulated {
		fmt.Println("  (Execute a simulação primeiro — opção 8)")
		return
	}
	for _, proc := range s.Processes {
		fmt.Printf("\n  Tabela de Páginas — %s (ID=%d)\n", proc.Name, proc.ID)
		const w1, w2, w3, w4 = 6, 8, 6, 8
		line := fmt.Sprintf("  +%s+%s+%s+%s+", sep("-", w1+2), sep("-", w2+2), sep("-", w3+2), sep("-", w4+2))
		fmt.Println(line)
		fmt.Printf("  | %s | %s | %s | %s |\n",
			centerPad("Pág.", w1), centerPad("Quadro", w2),
			centerPad("Válid.", w3), centerPad("Na Mem.", w4))
		fmt.Println(line)
		entries := s.PageTables[proc.ID]
		for i, e := range entries {
			frame := "-"
			valid := "não"
			inMem := "não"
			if e.FrameNum != -1 {
				frame = fmt.Sprintf("%d", e.FrameNum)
			}
			if e.Valid {
				valid = "sim"
				inMem = "sim"
			}
			fmt.Printf("  | %s | %s | %s | %s |\n",
				centerPad(fmt.Sprintf("%d", i), w1),
				centerPad(frame, w2),
				centerPad(valid, w3),
				centerPad(inMem, w4))
		}
		fmt.Println(line)
	}
}

func (s *PagingState) showFragmentation() {
	if !s.Simulated {
		fmt.Println("  (Execute a simulação primeiro — opção 8)")
		return
	}
	fmt.Println("\n  ─── Relatório de Fragmentação (Paginação) ───")
	totalInternal := 0
	for _, proc := range s.Processes {
		intFrag := (proc.Pages*s.PageSize - proc.Size)
		totalInternal += intFrag
		fmt.Printf("  %s: %d págs × %d %s - %d %s = %d %s de frag. interna\n",
			proc.Name, proc.Pages, s.PageSize, s.Unit, proc.Size, s.Unit, intFrag, s.Unit)
	}
	fmt.Printf("  Total frag. interna  : %d %s\n", totalInternal, s.Unit)
	fmt.Printf("  Fragmentação externa : N/A — paginação elimina fragmentação externa\n")

	// free frames
	free := 0
	for _, f := range s.Frames {
		if f.ProcID == -1 {
			free++
		}
	}
	fmt.Printf("  Quadros livres       : %d / %d\n", free, s.numFrames())
}

func (s *PagingState) showPageFaultStats() {
	if len(s.PageFaults) == 0 {
		fmt.Println("  (Execute a simulação primeiro — opção 8)")
		return
	}
	const w1, w2 = 12, 14
	line := fmt.Sprintf("+%s+%s+", sep("-", w1+2), sep("-", w2+2))
	fmt.Println(line)
	fmt.Printf("| %s | %s |\n", centerPad("Algoritmo", w1), centerPad("Page Faults", w2))
	fmt.Println(line)
	for _, algo := range []string{"fifo", "lru", "clock"} {
		faults := s.PageFaults[algo]
		label := strings.ToUpper(algo)
		if algo == "clock" {
			label = "Clock (SC)"
		}
		fmt.Printf("| %s | %s |\n",
			rpad(label, w1),
			centerPad(fmt.Sprintf("%d", faults), w2))
	}
	fmt.Println(line)
}

func (s *PagingState) listProcesses() {
	if len(s.Processes) == 0 {
		fmt.Println("  (nenhum processo)")
		return
	}
	fmt.Printf("  %-4s %-14s %-8s %-6s\n", "ID", "Nome", "Tam.", "Págs.")
	fmt.Printf("  %s\n", sep("-", 40))
	for _, p := range s.Processes {
		fmt.Printf("  %-4d %-14s %-8d %-6d\n", p.ID, p.Name, p.Size, p.Pages)
	}
}

func menuPaging(s *PagingState) {
	for {
		fmt.Printf("\n╔══════════════════════════════════════╗\n")
		fmt.Printf("║         PAGINAÇÃO                    ║\n")
		fmt.Printf("╚══════════════════════════════════════╝\n")
		fmt.Printf("  Config: Física=%d%s  Virtual=%d%s  Página=%d%s  Algo=%s\n",
			s.PhysMem, s.Unit, s.VirtMem, s.Unit, s.PageSize, s.Unit, strings.ToUpper(s.Algorithm))
		fmt.Printf("  Quadros: %d  |  Processos: %d\n", s.numFrames(), len(s.Processes))
		fmt.Println()
		fmt.Println("  1.  Definir tamanho da memória física")
		fmt.Println("  2.  Definir tamanho da memória virtual")
		fmt.Println("  3.  Definir tamanho da página")
		fmt.Println("  4.  Definir algoritmo de substituição")
		fmt.Println("  5.  Definir intervalo K (Clock reset)")
		fmt.Println("  6.  Adicionar processo")
		fmt.Println("  7.  Remover processo")
		fmt.Println("  8.  Executar simulação de paginação")
		fmt.Println("  9.  Mostrar tabela de quadros")
		fmt.Println("  10. Mostrar tabelas de páginas")
		fmt.Println("  11. Mostrar relatório de fragmentação")
		fmt.Println("  12. Mostrar estatísticas de page faults")
		fmt.Println("  13. Listar processos")
		fmt.Println("  14. Reiniciar")
		fmt.Println("  0.  Voltar")
		fmt.Println()

		choice := readLine("  Opção: ")
		switch choice {
		case "1":
			s.PhysMem = readIntMin("  Memória física: ", 1)
			s.Unit = readLine("  Unidade: ")
			if s.Unit == "" {
				s.Unit = "MB"
			}
			s.Simulated = false
		case "2":
			v := readIntMin("  Memória virtual: ", 1)
			if v <= s.PhysMem {
				fmt.Printf("  ✗ Memória virtual (%d) deve ser maior que a física (%d).\n", v, s.PhysMem)
			} else {
				s.VirtMem = v
				s.Simulated = false
			}
		case "3":
			v := readIntMin("  Tamanho da página: ", 1)
			if s.PhysMem%v != 0 || s.VirtMem%v != 0 {
				fmt.Println("  ✗ O tamanho de página deve dividir exatamente tanto a memória física quanto a virtual.")
			} else {
				s.PageSize = v
				// update page counts
				for _, p := range s.Processes {
					p.Pages = int(math.Ceil(float64(p.Size) / float64(s.PageSize)))
				}
				s.Simulated = false
			}
		case "4":
			fmt.Println("  1. FIFO")
			fmt.Println("  2. LRU")
			fmt.Println("  3. Clock (SC)")
			ac := readLine("  Escolha: ")
			switch ac {
			case "1":
				s.Algorithm = "fifo"
			case "2":
				s.Algorithm = "lru"
			case "3":
				s.Algorithm = "clock"
			default:
				fmt.Println("  ✗ Opção inválida.")
			}
		case "5":
			k := readIntMin("  Intervalo K (0 = usar nº de quadros): ", 0)
			s.ClockK = k
		case "6":
			if err := s.validate(); err != nil {
				fmt.Printf("  ✗ Configuração inválida: %s\n", err)
				continue
			}
			name := readLine("  Nome do processo: ")
			if name == "" {
				fmt.Println("  ✗ Nome não pode ser vazio.")
				continue
			}
			size := readIntMin("  Tamanho ("+s.Unit+"): ", 1)
			s.addProcess(name, size)
		case "7":
			s.listProcesses()
			id := readInt("  ID do processo a remover: ")
			found := false
			for i, p := range s.Processes {
				if p.ID == id {
					s.Processes = append(s.Processes[:i], s.Processes[i+1:]...)
					fmt.Printf("  ✓ Processo ID=%d removido.\n", id)
					s.Simulated = false
					found = true
					break
				}
			}
			if !found {
				fmt.Println("  ✗ Processo não encontrado.")
			}
		case "8":
			s.runSimulation()
		case "9":
			s.showFrameTable()
		case "10":
			s.showPageTables()
		case "11":
			s.showFragmentation()
		case "12":
			s.showPageFaultStats()
		case "13":
			s.listProcesses()
		case "14":
			s.Processes = nil
			s.NextProcID = 1
			s.Simulated = false
			s.PageFaults = make(map[string]int)
			fmt.Println("  ✓ Estado reiniciado.")
		case "0":
			return
		default:
			fmt.Println("  ✗ Opção inválida.")
		}
	}
}

// ─────────────────────────────────────────────
// main
// ─────────────────────────────────────────────

func main() {
	rand.Seed(time.Now().UnixNano()) //nolint:staticcheck

	partState := newPartitionState()
	pagState := newPagingState()

	for {
		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════╗")
		fmt.Println("║    SIMULADOR DE GERENCIAMENTO DE MEMÓRIA     ║")
		fmt.Println("╚══════════════════════════════════════════════╝")
		fmt.Println("  1. Alocação por Partições (Variáveis)")
		fmt.Println("  2. Paginação com Substituição de Páginas")
		fmt.Println("  0. Sair")
		fmt.Println()

		choice := readLine("  Opção: ")
		switch choice {
		case "1":
			menuPartition(partState)
		case "2":
			menuPaging(pagState)
		case "0":
			fmt.Println("  Saindo... até logo!")
			return
		default:
			fmt.Println("  ✗ Opção inválida.")
		}
	}
}

/*
Build & Run:
    go run main.go
    -- or --
    go build -o memsim && ./memsim
Requirements: Go 1.21+, no external dependencies.
*/