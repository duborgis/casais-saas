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
    <div className="min-h-screen bg-[#fdfaf5] flex flex-col items-center p-4 font-sans">
      {/* Header */}
      <header className="w-full max-w-2xl bg-white/80 backdrop-blur-md rounded-3xl shadow-sm p-6 mb-8 flex items-center justify-between border border-orange-100/50">
        <div className="flex items-center gap-4">
          <div className="bg-gradient-to-br from-rose-400 to-orange-300 p-3 rounded-2xl shadow-rose-200/50 shadow-lg text-white">
            <Heart className="w-6 h-6 fill-current" />
          </div>
          <div>
            <h1 className="text-2xl font-black text-slate-800 tracking-tight leading-none m-0">Harmony</h1>
            <p className="text-sm font-medium text-rose-400/80 mt-1 m-0 italic">Mediacao Inteligente</p>
          </div>
        </div>
        <div className="hidden sm:flex items-center gap-2 bg-emerald-50 px-4 py-2 rounded-2xl border border-emerald-100/50">
          <ShieldCheck className="text-emerald-500 w-4 h-4" />
          <span className="text-[10px] font-bold text-emerald-700 tracking-widest uppercase">Encriptado</span>
        </div>
      </header>

      {/* Chat Area */}
      <main className="w-full max-w-2xl bg-white rounded-[2.5rem] shadow-xl shadow-rose-900/5 border border-rose-100/30 flex-1 flex flex-col overflow-hidden mb-6 min-h-[550px]">
        <div ref={scrollRef} className="flex-1 overflow-y-auto p-8 space-y-6 scroll-smooth">
          <AnimatePresence initial={false}>
            {messages.map((msg) => (
              <motion.div
                key={msg.id}
                initial={{ opacity: 0, scale: 0.95, y: 10 }}
                animate={{ opacity: 1, scale: 1, y: 0 }}
                className={`flex ${msg.sender === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                <div 
                  className={`max-w-[85%] px-6 py-4 rounded-[2rem] shadow-sm ${
                    msg.sender === 'user' 
                      ? 'bg-gradient-to-br from-rose-500 to-rose-600 text-white rounded-tr-none' 
                      : msg.sender === 'agent'
                      ? 'bg-slate-50 text-slate-700 rounded-tl-none border border-slate-100'
                      : 'bg-orange-50 text-orange-800 rounded-tl-none border border-orange-100'
                  }`}
                >
                  <p className="text-[0.95rem] leading-relaxed font-medium m-0 whitespace-pre-wrap">{msg.text}</p>
                </div>
              </motion.div>
            ))}
          </AnimatePresence>
          {isTyping && (
            <div className="flex justify-start">
               <div className="bg-slate-50 px-5 py-3 rounded-[1.5rem] rounded-tl-none border border-slate-100 flex items-center gap-2">
                 <div className="flex gap-1">
                   <span className="w-1.5 h-1.5 bg-rose-400 rounded-full animate-bounce [animation-delay:-0.3s]"></span>
                   <span className="w-1.5 h-1.5 bg-rose-400 rounded-full animate-bounce [animation-delay:-0.15s]"></span>
                   <span className="w-1.5 h-1.5 bg-rose-400 rounded-full animate-bounce"></span>
                 </div>
                 <span className="text-[10px] font-bold text-slate-400 tracking-wider uppercase">Harmony ouvindo...</span>
               </div>
            </div>
          )}
        </div>

        {/* Input */}
        <div className="p-6 bg-slate-50/50 border-t border-rose-50 flex items-center gap-4">
          <div className="flex-1 relative group">
            <input
              type="text"
              value={input}
              disabled={isTyping}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && sendMessage()}
              placeholder={isTyping ? "Pausado para reflexao..." : "O que esta em seu coracao?"}
              className="w-full bg-white border border-rose-100/50 rounded-2xl px-6 py-4 focus:outline-none focus:ring-4 focus:ring-rose-500/10 focus:border-rose-300 transition-all text-slate-800 placeholder:text-slate-300 font-medium disabled:opacity-50"
            />
          </div>
          <button 
            onClick={sendMessage}
            disabled={isTyping || !input.trim()}
            className="bg-rose-500 hover:bg-rose-600 disabled:bg-slate-200 text-white p-4 rounded-2xl shadow-lg shadow-rose-200 transition-all hover:scale-105 active:scale-95 disabled:shadow-none"
          >
            <Send className="w-6 h-6" />
          </button>
        </div>
      </main>

      {/* Footer / Info */}
      <footer className="w-full max-w-2xl flex flex-col items-center gap-2 pb-8">
        <div className="flex items-center gap-3 bg-white px-5 py-2 rounded-full shadow-sm border border-rose-50/50">
          <Sparkles className="w-4 h-4 text-orange-400 fill-current" />
          <span className="text-[10px] font-black text-slate-400 tracking-[0.2em] uppercase">IA de Conexao Humana</span>
        </div>
        <p className="text-[10px] text-slate-300 font-medium text-center max-w-xs mt-2 leading-relaxed">
          Harmony utiliza inteligencia artificial de ultima geracao para facilitar a comunicacao nao-violenta entre parceiros.
        </p>
      </footer>
    </div>
  );
};

export default App;
