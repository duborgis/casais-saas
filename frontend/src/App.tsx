import React, { useState, useEffect, useRef } from 'react';
import { Heart, Send, Sparkles, ShieldCheck } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { io, Socket } from 'socket.io-client';

interface Message {
  id: string;
  sender: 'user' | 'agent' | 'partner';
  text: string;
}

const App: React.FC = () => {
  const [messages, setMessages] = useState<Message[]>([
    { id: '1', sender: 'agent', text: 'Olá! Sou o Harmony, seu mediador inteligente. Como posso ajudar vocês hoje?' }
  ]);
  const [input, setInput] = useState('');
  const [isTyping, setIsTyping] = useState(false);
  const socketRef = useRef<Socket | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // Conectar ao backend do Casais SaaS
    socketRef.current = io('http://localhost:3001');

    socketRef.current.on('agent_delta', (data: { delta: string }) => {
      setIsTyping(true);
      setMessages(prev => {
        const lastMsg = prev[prev.length - 1];
        if (lastMsg && lastMsg.sender === 'agent' && lastMsg.id === 'typing-agent') {
          return [...prev.slice(0, -1), { ...lastMsg, text: lastMsg.text + data.delta }];
        } else {
          return [...prev, { id: 'typing-agent', sender: 'agent', text: data.delta }];
        }
      });
    });

    socketRef.current.on('agent_complete', (data: { fullText: string }) => {
      setIsTyping(false);
      setMessages(prev => {
        const filtered = prev.filter(m => m.id !== 'typing-agent');
        return [...filtered, { id: Date.now().toString(), sender: 'agent', text: data.fullText }];
      });
    });

    socketRef.current.on('error', (data: { message: string }) => {
      alert(data.message);
      setIsTyping(false);
    });

    return () => {
      socketRef.current?.disconnect();
    };
  }, []);

  useEffect(() => {
    scrollRef.current?.scrollTo(0, scrollRef.current.scrollHeight);
  }, [messages]);

  const sendMessage = () => {
    if (!input.trim() || isTyping) return;
    
    const newUserMsg: Message = { id: Date.now().toString(), sender: 'user', text: input };
    setMessages(prev => [...prev, newUserMsg]);
    
    socketRef.current?.emit('message', { text: input, userId: 'user-1' });
    setInput('');
    setIsTyping(true);
  };

  return (
    <div className="min-h-screen bg-slate-50 flex flex-col items-center p-4">
      {/* Header */}
      <header className="w-full max-w-2xl bg-white rounded-2xl shadow-sm p-6 mb-6 flex items-center justify-between border border-slate-100">
        <div className="flex items-center gap-3">
          <div className="bg-purple-100 p-3 rounded-full">
            <Heart className="text-purple-600 w-6 h-6" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-slate-800 m-0 leading-none">Harmony Mediation</h1>
            <p className="text-sm text-slate-500 mt-1 m-0">Espaço seguro para casais</p>
          </div>
        </div>
        <div className="hidden sm:flex items-center gap-2 bg-green-50 px-3 py-1 rounded-full">
          <ShieldCheck className="text-green-600 w-4 h-4" />
          <span className="text-xs font-medium text-green-700 uppercase">Seguro & Privado</span>
        </div>
      </header>

      {/* Chat Area */}
      <main className="w-full max-w-2xl bg-white rounded-3xl shadow-lg border border-slate-100 flex-1 flex flex-col overflow-hidden mb-4 min-h-[500px]">
        <div ref={scrollRef} className="flex-1 overflow-y-auto p-6 space-y-4">
          <AnimatePresence>
            {messages.map((msg) => (
              <motion.div
                key={msg.id}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                className={`flex ${msg.sender === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                <div 
                  className={`max-w-[85%] p-4 rounded-2xl ${
                    msg.sender === 'user' 
                      ? 'bg-purple-600 text-white rounded-tr-none' 
                      : msg.sender === 'agent'
                      ? 'bg-slate-100 text-slate-700 rounded-tl-none border border-slate-200'
                      : 'bg-pink-100 text-pink-700 rounded-tl-none'
                  }`}
                >
                  <p className="text-sm leading-relaxed m-0 whitespace-pre-wrap">{msg.text}</p>
                </div>
              </motion.div>
            ))}
          </AnimatePresence>
          {isTyping && (
            <div className="flex justify-start">
               <div className="bg-slate-100 p-3 rounded-2xl rounded-tl-none animate-pulse">
                 <span className="text-xs text-slate-400">Harmony está pensando...</span>
               </div>
            </div>
          )}
        </div>

        {/* Input */}
        <div className="p-4 bg-slate-50 border-t border-slate-100 flex items-center gap-3">
          <input
            type="text"
            value={input}
            disabled={isTyping}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && sendMessage()}
            placeholder={isTyping ? "Aguarde a resposta..." : "Digite algo para mediar..."}
            className="flex-1 bg-white border border-slate-200 rounded-xl px-4 py-3 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all text-slate-800 disabled:bg-slate-100 disabled:cursor-not-allowed"
          />
          <button 
            onClick={sendMessage}
            disabled={isTyping}
            className="bg-purple-600 hover:bg-purple-700 disabled:bg-slate-300 text-white p-3 rounded-xl transition-colors"
          >
            <Send className="w-5 h-5" />
          </button>
        </div>
      </main>

      {/* Footer / Tip */}
      <footer className="w-full max-w-2xl flex items-center justify-center gap-2 text-slate-400 text-sm pb-4">
        <Sparkles className="w-4 h-4 text-purple-400" />
        <span>IA treinada em mediação de conflitos e comunicação não-violenta</span>
      </footer>
    </div>
  );
};

export default App;
