# Aircraft IoT Simulation Network

Este projeto é uma simulação de rede de sensores e atuadores para uma aeronave, baseada em uma arquitetura Cliente-Servidor. Ele permite o monitoramento de dados de telemetria em tempo real e o controle remoto de componentes da aeronave através de um painel de usuário centralizado.

## Estrutura do Projeto

O sistema é composto por quatro módulos principais, cada um empacotado em sua própria imagem Docker:

* **Servidor Central (`aircraft-server`)**: O núcleo (broker) da rede. Ele gerencia conexões concorrentes, mantém o estado atual de todos os dispositivos e atua como ponte de comunicação entre os nós.
* **Sensores (`aircraft-anemo` e `aircraft-fuel`)**: Clientes emissores de dados contínuos. O Anemômetro simula a velocidade do vento, enquanto o sensor de Combustível simula o consumo gradual.
* **Atuadores (`aircraft-siren`)**: Clientes receptores. A sirene é um componente de hardware simulado que aguarda comandos para ligar/desligar e retorna feedback do seu estado real.
* **Painel do Usuário (`aircraft-user`)**: Interface de Terminal (TUI) interativa que permite a um operador humano visualizar a lista de dispositivos conectados, realizar streaming de dados dos sensores e enviar comandos aos atuadores.

---

## Fluxo de Dados e Comunicação

A rede utiliza dois protocolos distintos dependendo da natureza do componente:

1.  **Sensores (UDP):** Projetados para alta taxa de transferência e tolerância a perdas. Os sensores enviam pacotes de dados brutos (ex: `ANEMO/24.50`) de forma contínua e unidirecional para o Servidor. O servidor atualiza o estado interno sempre com o dado mais recente.
2.  **Atuadores e Usuários (TCP):** Projetados para confiabilidade de entrega. Utilizam um processo de *Handshake* (`HND`) inicial.
    * **Usuários:** Requisitam a lista de dispositivos disponíveis (`LST`/`LSA`), pedem conexão de streaming contínuo com sensores específicos (`GET`) ou enviam comandos de alteração de estado para os atuadores (`SST`).
    * **Atuadores:** Mantêm uma conexão passiva aguardando comandos (`ON`/`OFF`) repassados pelo Servidor. Ao executarem uma ação, retornam uma confirmação de estado (`FDB`) para manter o painel do usuário sincronizado.

---

## Como Utilizar (Guia de Execução)

Este guia assume um cenário de rede distribuída: o **Servidor** será executado em uma máquina central (Máquina A), enquanto os **Sensores, Atuadores e Usuários** serão executados em uma ou mais máquinas clientes (Máquina B, C, etc.).

### 1. Download das Imagens (Pull)
Em todas as máquinas que forem executar qualquer parte do sistema, você deve baixar as imagens pré-compiladas diretamente do Docker Hub. No terminal, execute:

```bash
docker pull bdaemonis/aircraft-server:latest
docker pull bdaemonis/aircraft-anemo:latest
docker pull bdaemonis/aircraft-fuel:latest
docker pull bdaemonis/aircraft-siren:latest
docker pull bdaemonis/aircraft-user:latest
