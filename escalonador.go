package main

import (
	"bufio"
	"container/list"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ===================== TIPOS E ESTRUTURAS =====================

type TipoProcesso int

const (
	CPU_BOUND TipoProcesso = iota
	IO_BOUND
)

func (t TipoProcesso) String() string {
	if t == CPU_BOUND {
		return "CPU-Bound"
	}
	return "I/O-Bound"
}

type EstadoProcesso int

const (
	NOVO EstadoProcesso = iota
	PRONTO
	EXECUTANDO
	BLOQUEADO
	FINALIZADO
)

func (e EstadoProcesso) String() string {
	switch e {
	case NOVO:
		return "NOVO"
	case PRONTO:
		return "PRONTO"
	case EXECUTANDO:
		return "EXECUTANDO"
	case BLOQUEADO:
		return "BLOQUEADO"
	case FINALIZADO:
		return "FINALIZADO"
	}
	return "DESCONHECIDO"
}

type Processo struct {
	ID             int
	Nome           string
	Prioridade     int
	Tipo           TipoProcesso
	TempoTotalCPU  int
	TempoRestante  int
	TempoExecutado int
	Estado         EstadoProcesso
	TempoEspera    int 

	chegadaSim     int
	inicioSim      int
	inicioDefinido bool
	fimSim         int
	entradaFila    int 

	rajadaCPU       int
	tempoBloqueioIO int
	ciclosIO        int
}

func fatorIO(quantum int) (rajada, bloqueio int) {
	rajada = quantum * 30 / 100
	if rajada < 1 {
		rajada = 1
	}
	return rajada, rajada
}

func NovoProcesso(id int, nome string, prioridade int, tipo TipoProcesso, tempoCPU, quantum, tempoAtual int) *Processo {
	p := &Processo{
		ID:            id,
		Nome:          nome,
		Prioridade:    prioridade,
		Tipo:          tipo,
		TempoTotalCPU: tempoCPU,
		TempoRestante: tempoCPU,
		Estado:        NOVO,
		chegadaSim:    tempoAtual,
	}
	if tipo == IO_BOUND {
		p.rajadaCPU, p.tempoBloqueioIO = fatorIO(quantum)
	}
	return p
}

func (p *Processo) Turnaround() int {
	if p.Estado == FINALIZADO {
		return p.fimSim - p.chegadaSim
	}
	return 0
}

type AlgoritmoEscalonamento int

const (
	ROUND_ROBIN AlgoritmoEscalonamento = iota
	PRIORIDADE
)

func (a AlgoritmoEscalonamento) String() string {
	if a == ROUND_ROBIN {
		return "Round Robin"
	}
	return "Prioridade (Preemptivo)"
}

// ===================== ESCALONADOR =====================

type Escalonador struct {
	algoritmo     AlgoritmoEscalonamento
	quantum       int
	processos     map[int]*Processo
	filaProntos   *list.List
	processoAtual *Processo
	mutex         sync.Mutex
	emExecucao    bool
	tempoSimulado int
	logExecucao   []string
}

func NovoEscalonador(algo AlgoritmoEscalonamento, quantum int) *Escalonador {
	return &Escalonador{
		algoritmo:   algo,
		quantum:     quantum,
		processos:   make(map[int]*Processo),
		filaProntos: list.New(),
	}
}

// insereNaFila deve ser chamada com mutex adquirido.
func (e *Escalonador) insereNaFila(p *Processo) {
	p.Estado = PRONTO
	p.entradaFila = e.tempoSimulado

	if e.algoritmo == PRIORIDADE {
		inserido := false
		for elem := e.filaProntos.Front(); elem != nil; elem = elem.Next() {
			proc := elem.Value.(*Processo)
			if p.Prioridade < proc.Prioridade {
				e.filaProntos.InsertBefore(p, elem)
				inserido = true
				break
			}
		}
		if !inserido {
			e.filaProntos.PushBack(p)
		}
	} else {
		e.filaProntos.PushBack(p)
	}
}

func (e *Escalonador) AdicionarProcesso(p *Processo) {
	e.mutex.Lock()
	e.processos[p.ID] = p
	e.insereNaFila(p)
	e.mutex.Unlock()
	fmt.Printf("+ Processo %s (ID=%d) adicionado a fila de PRONTOS\n", p.Nome, p.ID)
}

// proximoProcesso deve ser chamada com mutex adquirido.
func (e *Escalonador) proximoProcesso() *Processo {
	elem := e.filaProntos.Front()
	if elem == nil {
		return nil
	}
	proc := elem.Value.(*Processo)
	e.filaProntos.Remove(elem)

	cicloEspera := e.tempoSimulado - proc.entradaFila
	if cicloEspera > 0 {
		proc.TempoEspera += cicloEspera
	}
	return proc
}

func (e *Escalonador) snapshotFila() []string {
	var nomes []string
	for elem := e.filaProntos.Front(); elem != nil; elem = elem.Next() {
		p := elem.Value.(*Processo)
		nomes = append(nomes, fmt.Sprintf("%s(ID=%d,P=%d)", p.Nome, p.ID, p.Prioridade))
	}
	return nomes
}

func mostrarSnapshot(snap []string) {
	if len(snap) == 0 {
		fmt.Println("[Fila Prontos] (vazia)")
	} else {
		fmt.Printf("[Fila Prontos] %s\n", strings.Join(snap, " -> "))
	}
}

func (e *Escalonador) mostrarFilaProntos() {
	e.mutex.Lock()
	snap := e.snapshotFila()
	e.mutex.Unlock()
	mostrarSnapshot(snap)
}

// ===================== LOOP PRINCIPAL =====================

func (e *Escalonador) Executar() {
	e.mutex.Lock()
	if e.emExecucao {
		e.mutex.Unlock()
		fmt.Println("Escalonador ja esta em execucao!")
		return
	}
	e.emExecucao = true
	e.mutex.Unlock()

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  INICIANDO EXECUCAO DO ESCALONADOR")
	fmt.Printf("  Algoritmo: %s | Quantum: %d ms\n", e.algoritmo, e.quantum)
	fmt.Println(strings.Repeat("=", 70))

	for {
		e.mutex.Lock()

		if e.filaProntos.Len() == 0 {
			todosFinalizados := true
			for _, p := range e.processos {
				if p.Estado != FINALIZADO {
					todosFinalizados = false
					break
				}
			}
			if todosFinalizados && len(e.processos) > 0 {
				e.mutex.Unlock()
				fmt.Println("\n" + strings.Repeat("=", 70))
				fmt.Println("  TODOS OS PROCESSOS FINALIZADOS!")
				fmt.Println(strings.Repeat("=", 70))
				break
			}
			e.mutex.Unlock()
			time.Sleep(50 * time.Millisecond)
			continue
		}

		processo := e.proximoProcesso()
		if processo == nil {
			e.mutex.Unlock()
			continue
		}

		if !processo.inicioDefinido {
			processo.inicioSim = e.tempoSimulado
			processo.inicioDefinido = true
		}

		processo.Estado = EXECUTANDO
		e.processoAtual = processo
		snap := e.snapshotFila()

		e.mutex.Unlock()

		mostrarSnapshot(snap)
		e.executarProcesso(processo)
	}

	e.emExecucao = false
}

// ===================== EXECUCAO DE UM PROCESSO =====================

func (e *Escalonador) executarProcesso(p *Processo) {
	var tempoExecucao int
	if p.Tipo == IO_BOUND && p.rajadaCPU > 0 {
		tempoExecucao = p.rajadaCPU
		if p.TempoRestante < tempoExecucao {
			tempoExecucao = p.TempoRestante
		}
	} else {
		tempoExecucao = e.quantum
		if p.TempoRestante < tempoExecucao {
			tempoExecucao = p.TempoRestante
		}
	}

	logMsg := fmt.Sprintf("\n[CPU] %s (ID=%d, Prio=%d, %s) | restante=%d ms | t=%d ms",
		p.Nome, p.ID, p.Prioridade, p.Tipo, p.TempoRestante, e.tempoSimulado)
	e.logExecucao = append(e.logExecucao, logMsg)
	fmt.Println(logMsg)
	fmt.Printf("[CPU] Executando por %d ms...\n", tempoExecucao)

	time.Sleep(time.Duration(tempoExecucao) * time.Millisecond)

	e.mutex.Lock()

	p.TempoExecutado += tempoExecucao
	p.TempoRestante -= tempoExecucao
	e.tempoSimulado += tempoExecucao

	switch {
	case p.TempoRestante <= 0:
		p.Estado = FINALIZADO
		p.fimSim = e.tempoSimulado
		logMsg = fmt.Sprintf("[CPU] OK %s (ID=%d) FINALIZADO | turnaround=%d ms | espera=%d ms",
			p.Nome, p.ID, p.Turnaround(), p.TempoEspera)
		e.logExecucao = append(e.logExecucao, logMsg)
		fmt.Println(logMsg)
		e.processoAtual = nil
		e.mutex.Unlock()

	case p.Tipo == IO_BOUND:
		p.ciclosIO++
		p.Estado = BLOQUEADO
		bloqueio := p.tempoBloqueioIO
		e.processoAtual = nil
		logMsg = fmt.Sprintf("[I/O] %s (ID=%d) BLOQUEADO %d ms (ciclo %d) | CPU restante=%d ms",
			p.Nome, p.ID, bloqueio, p.ciclosIO, p.TempoRestante)
		e.logExecucao = append(e.logExecucao, logMsg)
		fmt.Println(logMsg)
		e.mutex.Unlock()

		time.Sleep(time.Duration(bloqueio) * time.Millisecond)

		e.mutex.Lock()
		logMsg = fmt.Sprintf("[I/O] %s (ID=%d) I/O CONCLUIDO -> fila de PRONTOS (t=%d ms)",
			p.Nome, p.ID, e.tempoSimulado)
		e.logExecucao = append(e.logExecucao, logMsg)
		fmt.Println(logMsg)
		e.insereNaFila(p)
		e.mutex.Unlock()

	default:
		logMsg = fmt.Sprintf("[CPU] %s (ID=%d) PREEMPTADO | restante=%d ms",
			p.Nome, p.ID, p.TempoRestante)
		e.logExecucao = append(e.logExecucao, logMsg)
		fmt.Println(logMsg)
		e.insereNaFila(p)
		e.processoAtual = nil
		e.mutex.Unlock()
	}
}

// ===================== ESTATISTICAS =====================

func (e *Escalonador) MostrarEstatisticas() {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  ESTATISTICAS FINAIS  (tempos em ms simulados)")
	fmt.Println(strings.Repeat("=", 70))

	var totalTurnaround, totalEspera int
	count := 0

	fmt.Println("\nProcesso          | Tipo      | Turnaround | Espera | Execucao | Estado")
	fmt.Println(strings.Repeat("-", 75))

	ids := make([]int, 0, len(e.processos))
	for id := range e.processos {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		p := e.processos[id]
		t := p.Turnaround()
		totalTurnaround += t
		totalEspera += p.TempoEspera
		count++
		fmt.Printf("%-17s | %-9s | %10d | %6d | %8d | %s\n",
			fmt.Sprintf("%s(ID=%d)", p.Nome, p.ID),
			p.Tipo, t, p.TempoEspera, p.TempoExecutado, p.Estado)
	}

	if count > 0 {
		fmt.Println(strings.Repeat("-", 75))
		fmt.Printf("%-17s | %-9s | %10.1f | %6.1f | %8s |\n",
			"MEDIA", "",
			float64(totalTurnaround)/float64(count),
			float64(totalEspera)/float64(count),
			"")
	}
}

// ===================== INTERFACE DO USUARIO =====================

type Simulador struct {
	escalonador *Escalonador
	scanner     *bufio.Scanner
	proximoID   int
}

func NovoSimulador() *Simulador {
	return &Simulador{
		scanner:   bufio.NewScanner(os.Stdin),
		proximoID: 1,
	}
}

func (s *Simulador) mostrarMenuPrincipal() {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  SIMULADOR DE ESCALONAMENTO PREEMPTIVO DE PROCESSOS")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("1. Criar novo processo")
	fmt.Println("2. Configurar algoritmo de escalonamento")
	fmt.Println("3. Configurar quantum")
	fmt.Println("4. Ver fila de processos prontos")
	fmt.Println("5. Iniciar execucao")
	fmt.Println("6. Ver estatisticas")
	fmt.Println("0. Sair")
	fmt.Print("\nEscolha uma opcao: ")
}

func (s *Simulador) criarProcesso() {
	fmt.Println("\n--- Criar Novo Processo ---")

	fmt.Print("Nome do processo: ")
	s.scanner.Scan()
	nome := strings.TrimSpace(s.scanner.Text())
	if nome == "" {
		nome = fmt.Sprintf("Processo%d", s.proximoID)
	}

	fmt.Print("Prioridade (1-10, menor = maior prioridade): ")
	s.scanner.Scan()
	prioridade, _ := strconv.Atoi(strings.TrimSpace(s.scanner.Text()))
	if prioridade < 1 {
		prioridade = 5
	}
	if prioridade > 10 {
		prioridade = 10
	}

	fmt.Print("Tipo (1=CPU-Bound, 2=I/O-Bound): ")
	s.scanner.Scan()
	tipoStr := strings.TrimSpace(s.scanner.Text())
	var tipo TipoProcesso
	if tipoStr == "2" {
		tipo = IO_BOUND
	} else {
		tipo = CPU_BOUND
	}

	fmt.Print("Tempo total de CPU (1-10 ms): ")
	s.scanner.Scan()
	tempoCPU, _ := strconv.Atoi(strings.TrimSpace(s.scanner.Text()))
	if tempoCPU < 1 {
		tempoCPU = 1
	}
	if tempoCPU > 10 {
		tempoCPU = 10
	}

	if s.escalonador == nil {
		fmt.Println("\nErro: Configure o algoritmo e quantum primeiro (opcoes 2 e 3)!")
		return
	}

	tempoAtual := s.escalonador.tempoSimulado
	processo := NovoProcesso(s.proximoID, nome, prioridade, tipo, tempoCPU, s.escalonador.quantum, tempoAtual)
	s.proximoID++

	if tipo == IO_BOUND {
		fmt.Printf("  I/O-Bound: rajada CPU=%d ms | bloqueio I/O=%d ms\n",
			processo.rajadaCPU, processo.tempoBloqueioIO)
	}

	s.escalonador.AdicionarProcesso(processo)
}

func (s *Simulador) configurarAlgoritmo() {
	fmt.Println("\n--- Configurar Algoritmo ---")
	fmt.Println("1. Round Robin")
	fmt.Println("2. Prioridade (Preemptivo)")
	fmt.Print("Escolha: ")

	s.scanner.Scan()
	opcao := strings.TrimSpace(s.scanner.Text())

	var algo AlgoritmoEscalonamento
	if opcao == "2" {
		algo = PRIORIDADE
	} else {
		algo = ROUND_ROBIN
	}

	if s.escalonador != nil {
		quantum := s.escalonador.quantum
		processos := s.escalonador.processos
		ts := s.escalonador.tempoSimulado
		s.escalonador = NovoEscalonador(algo, quantum)
		s.escalonador.tempoSimulado = ts
		for _, p := range processos {
			s.escalonador.processos[p.ID] = p
			if p.Estado == PRONTO || p.Estado == NOVO {
				s.escalonador.mutex.Lock()
				s.escalonador.insereNaFila(p)
				s.escalonador.mutex.Unlock()
			}
		}
	} else {
		s.escalonador = NovoEscalonador(algo, 3)
	}

	fmt.Printf("Algoritmo configurado: %s\n", algo)
}

func (s *Simulador) configurarQuantum() {
	fmt.Println("\n--- Configurar Quantum ---")
	fmt.Print("Tempo de quantum (1-10 ms): ")

	s.scanner.Scan()
	quantum, _ := strconv.Atoi(strings.TrimSpace(s.scanner.Text()))
	if quantum < 1 {
		quantum = 1
	}
	if quantum > 10 {
		quantum = 10
	}

	if s.escalonador != nil {
		algo := s.escalonador.algoritmo
		processos := s.escalonador.processos
		ts := s.escalonador.tempoSimulado
		s.escalonador = NovoEscalonador(algo, quantum)
		s.escalonador.tempoSimulado = ts
		for _, p := range processos {
			s.escalonador.processos[p.ID] = p
			if p.Estado == PRONTO || p.Estado == NOVO {
				s.escalonador.mutex.Lock()
				s.escalonador.insereNaFila(p)
				s.escalonador.mutex.Unlock()
			}
		}
	} else {
		s.escalonador = NovoEscalonador(ROUND_ROBIN, quantum)
	}

	fmt.Printf("Quantum configurado: %d ms\n", quantum)
}

func (s *Simulador) verFilaProntos() {
	if s.escalonador == nil {
		fmt.Println("\nNenhum escalonador configurado!")
		return
	}

	fmt.Println("\n--- Fila de Processos Prontos ---")
	s.escalonador.mostrarFilaProntos()

	fmt.Println("\n--- Todos os Processos ---")
	fmt.Println("ID  | Nome           | Prio | Tipo      | CPU Total | Restante | Estado")
	fmt.Println(strings.Repeat("-", 72))
	ids := make([]int, 0, len(s.escalonador.processos))
	for id := range s.escalonador.processos {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		p := s.escalonador.processos[id]
		fmt.Printf("%-3d | %-14s | %-4d | %-9s | %-9d | %-8d | %s\n",
			p.ID, p.Nome, p.Prioridade, p.Tipo, p.TempoTotalCPU, p.TempoRestante, p.Estado)
	}
}

func (s *Simulador) iniciarExecucao() {
	if s.escalonador == nil {
		fmt.Println("\nErro: Configure o escalonador primeiro!")
		return
	}
	if len(s.escalonador.processos) == 0 {
		fmt.Println("\nErro: Nenhum processo criado!")
		return
	}
	if s.escalonador.filaProntos.Len() == 0 {
		fmt.Println("\nErro: Nenhum processo na fila de prontos!")
		return
	}
	s.escalonador.Executar()
	s.escalonador.MostrarEstatisticas()
}

func (s *Simulador) verEstatisticas() {
	if s.escalonador == nil {
		fmt.Println("\nNenhum escalonador configurado!")
		return
	}
	s.escalonador.MostrarEstatisticas()
}

func (s *Simulador) Run() {
	fmt.Println("Bem-vindo ao Simulador de Escalonamento!")
	fmt.Println("Configuracao inicial:")

	s.configurarAlgoritmo()
	s.configurarQuantum()

	// Processos pré-definidos
	pA := NovoProcesso(s.proximoID, "a", 1, CPU_BOUND, 9, s.escalonador.quantum, s.escalonador.tempoSimulado)
	s.proximoID++
	s.escalonador.AdicionarProcesso(pA)

	pB := NovoProcesso(s.proximoID, "b", 5, IO_BOUND, 6, s.escalonador.quantum, s.escalonador.tempoSimulado)
	s.proximoID++
	s.escalonador.AdicionarProcesso(pB)

	pC := NovoProcesso(s.proximoID, "c", 9, CPU_BOUND, 3, s.escalonador.quantum, s.escalonador.tempoSimulado)
	s.proximoID++
	s.escalonador.AdicionarProcesso(pC)

	for {
		s.mostrarMenuPrincipal()

		s.scanner.Scan()
		opcao := strings.TrimSpace(s.scanner.Text())

		switch opcao {
		case "1":
			s.criarProcesso()
		case "2":
			s.configurarAlgoritmo()
		case "3":
			s.configurarQuantum()
		case "4":
			s.verFilaProntos()
		case "5":
			s.iniciarExecucao()
		case "6":
			s.verEstatisticas()
		case "0":
			fmt.Println("\nSaindo...")
			return
		default:
			fmt.Println("\nOpcao invalida!")
		}

		fmt.Println("\nPressione ENTER para continuar...")
		s.scanner.Scan()
	}
}

// ===================== MAIN =====================

func main() {
	simulador := NovoSimulador()
	simulador.Run()
}
