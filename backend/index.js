import express from 'express';
import cors from 'cors';
import dotenv from 'dotenv';
import { Server } from 'socket.io';
import http from 'http';
import axios from 'axios';

dotenv.config();

const app = express();
const server = http.createServer(app);
const io = new Server(server, {
  cors: {
    origin: "*",
    methods: ["GET", "POST"]
  }
});

app.use(cors());
app.use(express.json());

const AGENT_BACKEND_URL = process.env.AGENT_BACKEND_URL || 'http://localhost:3005';
const AGENT_API_KEY = process.env.AGENT_API_KEY || 'default-secret';
const MEDIATOR_AGENT_ID = 'mediator-agent'; // Precisaremos criar este agente no backend-sse

app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'casais-saas-backend' });
});

// Proxy para o stream do Agent Backend
io.on('connection', (socket) => {
  console.log('[*] Novo usuário conectado no Harmony Mediation');

  socket.on('message', async (data) => {
    const { text, userId } = data;
    
    try {
      console.log(`[*] Enviando mensagem para o Agente: ${text}`);
      
      const response = await axios.post(`${AGENT_BACKEND_URL}/runs`, {
        agentId: MEDIATOR_AGENT_ID,
        message: text,
        userId: userId || 'anonymous'
      }, {
        headers: {
          'X-API-Key': AGENT_API_KEY,
          'Accept': 'text/event-stream'
        },
        responseType: 'stream'
      });

      let agentResponse = '';
      
      response.data.on('data', (chunk) => {
        const lines = chunk.toString().split('\n');
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const event = JSON.parse(line.slice(6));
              if (event.type === 'text_message_content') {
                agentResponse += event.delta;
                socket.emit('agent_delta', { delta: event.delta });
              } else if (event.type === 'run_finished') {
                socket.emit('agent_complete', { fullText: agentResponse });
              }
            } catch (e) {
              // Not JSON or partial
            }
          }
        }
      });

    } catch (error) {
      console.error('[!] Erro ao chamar agent-sse-backend:', error.message);
      socket.emit('error', { message: 'Falha na comunicação com o mediador.' });
    }
  });
});

const PORT = process.env.PORT || 3001;
server.listen(PORT, () => {
  console.log(`[OK] Backend Casais SaaS rodando na porta ${PORT}`);
  console.log(`[i] Integrado com: ${AGENT_BACKEND_URL}`);
});
