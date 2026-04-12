package main

import (
	"bufio"
	"container/list"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ===================== TIPOS E ESTRUTURAS =====================

// Tipo de processo
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

// Estado do processo
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

// Estrutura de um Processo
type Processo struct {
	ID              int
	Nome            string
	Prioridade      int
	Tipo            TipoProcesso
	TempoTotalCPU   int // ms
	TempoRestante   int // ms
	TempoExecutado  int // ms
	TempoChegada    time.Time
	TempoInicio     time.Time
	TempoFim        time.Time
	Estado          EstadoProcesso
	TempoEspera     int // ms
}

func NovoProcesso(id int, nome string, prioridade int, tipo TipoProcesso, tempoCPU int) *Processo {
	return &Processo{
		ID:             id,
		Nome:           nome,
		Prioridade:     prioridade,
		Tipo:           tipo,
		TempoTotalCPU:  tempoCPU,
		TempoRestante:  tempoCPU,
		TempoExecutado: 0,
		TempoChegada:   time.Now(),
		Estado:         NOVO,
		TempoEspera:    0,
	}
}

// Tempo de turnaround (tempo total desde a chegada até a finalização)
func (p *Processo) Turnaround() int {
	if p.Estado == FINALIZADO {
		return int(p.TempoFim.Sub(p.TempoChegada).Milliseconds())
	}
	return int(time.Since(p.TempoChegada).Milliseconds())
}

// Algoritmo de escalonamento
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
	algoritmo       AlgoritmoEscalonamento
	quantum         int // ms
	processos       map[int]*Processo
	filaProntos     *list.List
	processoAtual   *Processo
	mutex           sync.Mutex
	emExecucao      bool
	tempoSimulado   int // tempo simulado em ms
	logExecucao     []string
}

func NovoEscalonador(algo AlgoritmoEscalonamento, quantum int) *Escalonador {
	return &Escalonador{
		algoritmo:   algo,
		quantum:     quantum,
		processos:   make(map[int]*Processo),
		filaProntos: list.New(),
		emExecucao:  false,
		logExecucao: []string{},
	}
}

// Adiciona um processo à fila de prontos
func (e *Escalonador) AdicionarProcesso(p *Processo) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	p.Estado = PRONTO
	e.processos[p.ID] = p
	
	if e.algoritmo == PRIORIDADE {
		// Insere ordenado por prioridade (menor número = maior prioridade)
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
		// Round Robin - insere no final
		e.filaProntos.PushBack(p)
	}
	
	fmt.Printf("✓ Processo %s (ID=%d) adicionado à fila de PRONTOS\n", p.Nome, p.ID)
}

// Obtém o próximo processo da fila
func (e *Escalonador) proximoProcesso() *Processo {
	if e.filaProntos.Len() == 0 {
		return nil
	}
	
	elem := e.filaProntos.Front()
	if elem == nil {
		return nil
	}
	
	proc := elem.Value.(*Processo)
	e.filaProntos.Remove(elem)
	return proc
}

// Reinsere processo na fila de prontos (após preempção)
func (e *Escalonador) reinsereProcesso(p *Processo) {
	p.Estado = PRONTO
	
	if e.algoritmo == PRIORIDADE {
		// Reinsere ordenado por prioridade
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
		// Round Robin - insere no final
		e.filaProntos.PushBack(p)
	}
}

// Executa o escalonamento
func (e *Escalonador) Executar() {
	e.mutex.Lock()
	if e.emExecucao {
		e.mutex.Unlock()
		fmt.Println("Escalonador já está em execução!")
		return
	}
	e.emExecucao = true
	e.mutex.Unlock()
	
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  INICIANDO EXECUÇÃO DO ESCALONADOR")
	fmt.Printf("  Algoritmo: %s | Quantum: %d ms\n", e.algoritmo, e.quantum)
	fmt.Println(strings.Repeat("=", 70))
	
	// Atualiza tempo de início dos processos que vão executar
	for _, p := range e.processos {
		p.TempoInicio = time.Now()
	}
	
	for {
		e.mutex.Lock()
		
		// Verifica se há processos para executar
		if e.filaProntos.Len() == 0 {
			// Verifica se todos os processos finalizaram
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
			time.Sleep(100 * time.Millisecond)
			continue
		}
		
		// Pega o próximo processo
		processo := e.proximoProcesso()
		if processo == nil {
			e.mutex.Unlock()
			continue
		}
		
		e.processoAtual = processo
		processo.Estado = EXECUTANDO
		
		e.mutex.Unlock()
		
		// Executa o processo
		e.executarProcesso(processo)
	}
	
	e.emExecucao = false
}

// Executa um processo pelo quantum ou até finalizar
func (e *Escalonador) executarProcesso(p *Processo) {
	tempoExecucao := e.quantum
	if p.TempoRestante < tempoExecucao {
		tempoExecucao = p.TempoRestante
	}
	
	// Log de início de execução
	logMsg := fmt.Sprintf("\n[CPU] Processo %s (ID=%d, Prioridade=%d, %s) - Tempo restante: %d ms",
		p.Nome, p.ID, p.Prioridade, p.Tipo, p.TempoRestante)
	e.logExecucao = append(e.logExecucao, logMsg)
	fmt.Println(logMsg)
	
	// Mostra fila de prontos
	e.mostrarFilaProntos()
	
	// Simula execução
	fmt.Printf("[CPU] Executando por %d ms...\n", tempoExecucao)
	
	// Barra de progresso
	for i := 0; i < tempoExecucao; i += 100 {
		time.Sleep(100 * time.Millisecond)
		progresso := (i + 100) * 100 / tempoExecucao
		if progresso > 100 {
			progresso = 100
		}
		barra := strings.Repeat("█", progresso/5) + strings.Repeat("░", 20-progresso/5)
		fmt.Printf("\r[CPU] [%s] %d%%", barra, progresso)
	}
	fmt.Println()
	
	// Atualiza estatísticas
	e.mutex.Lock()
	p.TempoExecutado += tempoExecucao
	p.TempoRestante -= tempoExecucao
	e.tempoSimulado += tempoExecucao
	
	// Atualiza tempo de espera dos outros processos
	for elem := e.filaProntos.Front(); elem != nil; elem = elem.Next() {
		proc := elem.Value.(*Processo)
		proc.TempoEspera += tempoExecucao
	}
	
	if p.TempoRestante <= 0 {
		// Processo finalizado
		p.Estado = FINALIZADO
		p.TempoFim = time.Now()
		logMsg = fmt.Sprintf("[CPU] ✓ Processo %s (ID=%d) FINALIZADO - Turnaround: %d ms",
			p.Nome, p.ID, p.Turnaround())
		e.logExecucao = append(e.logExecucao, logMsg)
		fmt.Println(logMsg)
	} else {
		// Preempção - reinsere na fila
		logMsg = fmt.Sprintf("[CPU] ⚡ Processo %s (ID=%d) PREEMPTADO - Tempo restante: %d ms",
			p.Nome, p.ID, p.TempoRestante)
		e.logExecucao = append(e.logExecucao, logMsg)
		fmt.Println(logMsg)
		e.reinsereProcesso(p)
	}
	
	e.processoAtual = nil
	e.mutex.Unlock()
}

// Mostra a fila de processos prontos
func (e *Escalonador) mostrarFilaProntos() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	if e.filaProntos.Len() == 0 {
		fmt.Println("[Fila Prontos] (vazia)")
		return
	}
	
	var nomes []string
	for elem := e.filaProntos.Front(); elem != nil; elem = elem.Next() {
		p := elem.Value.(*Processo)
		nomes = append(nomes, fmt.Sprintf("%s(ID=%d,P=%d)", p.Nome, p.ID, p.Prioridade))
	}
	
	fmt.Printf("[Fila Prontos] %s\n", strings.Join(nomes, " -> "))
}

// Mostra estatísticas finais
func (e *Escalonador) MostrarEstatisticas() {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  ESTATÍSTICAS FINAIS")
	fmt.Println(strings.Repeat("=", 70))
	
	var totalTurnaround, totalEspera int
	count := 0
	
	fmt.Println("\nProcesso          | Turnaround | Tempo Espera | Estado")
	fmt.Println(strings.Repeat("-", 60))
	
	for _, p := range e.processos {
		turnaround := p.Turnaround()
		totalTurnaround += turnaround
		totalEspera += p.TempoEspera
		count++
		
		fmt.Printf("%-17s | %10d | %12d | %s\n",
			fmt.Sprintf("%s (ID=%d)", p.Nome, p.ID),
			turnaround,
			p.TempoEspera,
			p.Estado)
	}
	
	if count > 0 {
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("%-17s | %10.2f | %12.2f |\n",
			"MÉDIA",
			float64(totalTurnaround)/float64(count),
			float64(totalEspera)/float64(count))
	}
}

// ===================== INTERFACE DO USUÁRIO =====================

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

func (s *Simulador) limparTela() {
	fmt.Print("\033[H\033[2J")
}

func (s *Simulador) mostrarMenuPrincipal() {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  SIMULADOR DE ESCALONAMENTO PREEMPTIVO DE PROCESSOS")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Println("1. Criar novo processo")
	fmt.Println("2. Configurar algoritmo de escalonamento")
	fmt.Println("3. Configurar quantum")
	fmt.Println("4. Ver fila de processos prontos")
	fmt.Println("5. Iniciar execução")
	fmt.Println("6. Ver estatísticas")
	fmt.Println("0. Sair")
	fmt.Println()
	fmt.Print("Escolha uma opção: ")
}

func (s *Simulador) criarProcesso() {
	fmt.Println("\n--- Criar Novo Processo ---")
	
	// Nome
	fmt.Print("Nome do processo: ")
	s.scanner.Scan()
	nome := strings.TrimSpace(s.scanner.Text())
	if nome == "" {
		nome = fmt.Sprintf("Processo%d", s.proximoID)
	}
	
	// Prioridade
	fmt.Print("Prioridade (1-10, menor = maior prioridade): ")
	s.scanner.Scan()
	prioridade, _ := strconv.Atoi(strings.TrimSpace(s.scanner.Text()))
	if prioridade < 1 {
		prioridade = 5
	}
	if prioridade > 10 {
		prioridade = 10
	}
	
	// Tipo
	fmt.Print("Tipo (1=CPU-Bound, 2=I/O-Bound): ")
	s.scanner.Scan()
	tipoStr := strings.TrimSpace(s.scanner.Text())
	var tipo TipoProcesso
	if tipoStr == "2" {
		tipo = IO_BOUND
	} else {
		tipo = CPU_BOUND
	}
	
	// Tempo de CPU
	fmt.Print("Tempo total de CPU (1-10 ms): ")
	s.scanner.Scan()
	tempoCPU, _ := strconv.Atoi(strings.TrimSpace(s.scanner.Text()))
	if tempoCPU < 1 {
		tempoCPU = 1
	}
	if tempoCPU > 10 {
		tempoCPU = 10
	}
	
	// Cria o processo
	processo := NovoProcesso(s.proximoID, nome, prioridade, tipo, tempoCPU)
	s.proximoID++
	
	if s.escalonador == nil {
		fmt.Println("\nErro: Configure o algoritmo e quantum primeiro (opções 2 e 3)!")
		return
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
		// Preserva processos existentes
		quantum := s.escalonador.quantum
		processos := s.escalonador.processos
		s.escalonador = NovoEscalonador(algo, quantum)
		for _, p := range processos {
			s.escalonador.processos[p.ID] = p
			if p.Estado == PRONTO || p.Estado == NOVO {
				s.escalonador.reinsereProcesso(p)
			}
		}
	} else {
		s.escalonador = NovoEscalonador(algo, 3)
	}
	
	fmt.Printf("✓ Algoritmo configurado: %s\n", algo)
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
		s.escalonador = NovoEscalonador(algo, quantum)
		for _, p := range processos {
			s.escalonador.processos[p.ID] = p
			if p.Estado == PRONTO || p.Estado == NOVO {
				s.escalonador.reinsereProcesso(p)
			}
		}
	} else {
		s.escalonador = NovoEscalonador(ROUND_ROBIN, quantum)
	}
	
	fmt.Printf("✓ Quantum configurado: %d ms\n", quantum)
}

func (s *Simulador) verFilaProntos() {
	if s.escalonador == nil {
		fmt.Println("\nNenhum escalonador configurado!")
		return
	}
	
	fmt.Println("\n--- Fila de Processos Prontos ---")
	s.escalonador.mostrarFilaProntos()
	
	fmt.Println("\n--- Todos os Processos ---")
	fmt.Println("ID  | Nome           | Prioridade | Tipo      | Tempo Total | Tempo Restante | Estado")
	fmt.Println(strings.Repeat("-", 90))
	
	for _, p := range s.escalonador.processos {
		fmt.Printf("%-3d | %-14s | %-10d | %-9s | %-11d | %-14d | %s\n",
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
	// Configuração inicial
	fmt.Println("Bem-vindo ao Simulador de Escalonamento!")
	fmt.Println("Configuração inicial:")
	
	s.configurarAlgoritmo()
	s.configurarQuantum()
	
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
			fmt.Println("\nOpção inválida!")
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
