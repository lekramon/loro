export type DemoMovie = {
 id: string; title: string; year: number; minutes: number; genre: 'Science fiction' | 'Drama' | 'Adventure' | 'Thriller';
 rating: string; resolution: string; format: string; audio: string; synopsis: { 'en-US': string; 'pt-BR': string }; progress?: number;
};
export const demoMovies: DemoMovie[] = [
 {id:'orbit',title:'Orbit',year:2025,minutes:128,genre:'Science fiction',rating:'8.4',resolution:'2160p',format:'Remux',audio:'DTS-HD MA 5.1',progress:42,synopsis:{'en-US':'A lone navigator follows a mysterious signal beyond the last charted orbit. What waits in the silence could change the way humanity sees its home.','pt-BR':'Uma navegadora segue um sinal misterioso além da última órbita conhecida. O que a espera no silêncio pode mudar a forma como a humanidade vê seu lar.'}},
 {id:'deep-blue',title:'Deep Blue',year:2024,minutes:104,genre:'Adventure',rating:'7.8',resolution:'1080p',format:'WEB-DL',audio:'AAC 5.1',progress:18,synopsis:{'en-US':'An oceanographer returns to a remote coastal town and discovers a world beneath its quiet waters.','pt-BR':'Uma oceanógrafa retorna a uma cidade costeira e descobre um mundo sob suas águas tranquilas.'}},
 {id:'afterlight',title:'Afterlight',year:2025,minutes:112,genre:'Drama',rating:'8.1',resolution:'2160p',format:'WEB-DL',audio:'EAC3 5.1',synopsis:{'en-US':'Two strangers cross paths during the final night of a city that never sleeps.','pt-BR':'Dois desconhecidos se encontram durante a última noite de uma cidade que nunca dorme.'}},
 {id:'wild-north',title:'Wild North',year:2023,minutes:96,genre:'Adventure',rating:'7.6',resolution:'1080p',format:'Blu-ray',audio:'DTS 5.1',progress:63,synopsis:{'en-US':'A winter expedition becomes an unexpected journey of friendship and survival.','pt-BR':'Uma expedição de inverno se transforma em uma jornada inesperada de amizade e sobrevivência.'}},
 {id:'solstice',title:'Solstice',year:2025,minutes:119,genre:'Drama',rating:'8.0',resolution:'2160p',format:'Remux',audio:'TrueHD 7.1',synopsis:{'en-US':'Three generations reunite at a desert observatory on the longest day of the year.','pt-BR':'Três gerações se reúnem em um observatório no deserto no dia mais longo do ano.'}},
 {id:'silent-city',title:'Silent City',year:2024,minutes:107,genre:'Thriller',rating:'7.9',resolution:'1080p',format:'WEB-DL',audio:'EAC3 5.1',synopsis:{'en-US':'A sound engineer hears a message hidden in the noise of an empty city.','pt-BR':'Uma engenheira de som ouve uma mensagem escondida no ruído de uma cidade vazia.'}},
 {id:'beyond',title:'Beyond',year:2022,minutes:134,genre:'Science fiction',rating:'8.2',resolution:'2160p',format:'Blu-ray',audio:'DTS-HD MA 5.1',synopsis:{'en-US':'The crew of a research vessel must decide whether to return home or follow a discovery into the unknown.','pt-BR':'A tripulação de uma nave de pesquisa precisa escolher entre voltar para casa ou seguir uma descoberta rumo ao desconhecido.'}},
 {id:'the-crossing',title:'The Crossing',year:2023,minutes:101,genre:'Adventure',rating:'7.5',resolution:'1080p',format:'WEB-DL',audio:'AAC 5.1',synopsis:{'en-US':'A forgotten mountain route connects two lives separated by decades.','pt-BR':'Uma rota esquecida nas montanhas conecta duas vidas separadas por décadas.'}},
 {id:'golden-hour',title:'Golden Hour',year:2024,minutes:98,genre:'Drama',rating:'7.7',resolution:'1080p',format:'Blu-ray',audio:'DTS 5.1',synopsis:{'en-US':'A photographer takes one final road trip through the landscape of her childhood.','pt-BR':'Uma fotógrafa faz uma última viagem pelas paisagens de sua infância.'}},
 {id:'tides',title:'Tides',year:2025,minutes:110,genre:'Thriller',rating:'7.4',resolution:'2160p',format:'WEB-DL',audio:'EAC3 5.1',synopsis:{'en-US':'An unexpected arrival disrupts a lighthouse keeper’s carefully ordered life.','pt-BR':'Uma chegada inesperada interrompe a vida cuidadosamente organizada de um faroleiro.'}},
 {id:'evergreen',title:'Evergreen',year:2021,minutes:94,genre:'Adventure',rating:'7.8',resolution:'1080p',format:'Blu-ray',audio:'AAC 5.1',synopsis:{'en-US':'A young botanist follows a trail of letters into an ancient forest.','pt-BR':'Uma jovem botânica segue uma trilha de cartas até uma floresta ancestral.'}},
 {id:'last-signal',title:'Last Signal',year:2024,minutes:122,genre:'Science fiction',rating:'8.3',resolution:'2160p',format:'Remux',audio:'TrueHD 7.1',synopsis:{'en-US':'A radio operator receives a transmission from a station that disappeared thirty years ago.','pt-BR':'Uma operadora de rádio recebe uma transmissão de uma estação que desapareceu há trinta anos.'}},
];
export type DemoViewer = {id:string; name:string; color:string; favorites:string[]; watched:string[]; photo?:string};
export type DemoState = {version?:number; viewers:DemoViewer[]; activeId:string|null};
export const initialDemoState: DemoState = {activeId:null,viewers:[
 {id:'ramon',name:'Ramon',color:'#237eaa',favorites:['orbit','solstice'],watched:['afterlight']},
]};
const stateKey='piratarr.demo.viewers.v1';
export function readDemoState():DemoState {
 try {const saved:unknown=JSON.parse(localStorage.getItem(stateKey)??'null');
 if(saved && typeof saved==='object' && 'viewers' in saved && Array.isArray(saved.viewers) && saved.viewers.length>0 && saved.viewers.length<=12){
 const ids=new Set<string>();const valid=saved.viewers.every(v=>v && typeof v.id==='string' && /^[a-z0-9-]{1,60}$/.test(v.id) && !ids.has(v.id) && !!ids.add(v.id) && typeof v.name==='string' && v.name.trim().length>0 && v.name.length<=30 && typeof v.color==='string' && /^#[0-9a-f]{6}$/i.test(v.color) && Array.isArray(v.favorites) && v.favorites.every((x:unknown)=>typeof x==='string') && Array.isArray(v.watched) && v.watched.every((x:unknown)=>typeof x==='string'));
 if(valid) {
 const legacySeeds=[{id:'cinema',name:'Cinema',color:'#7951b4',favorites:['beyond'],watched:[]},{id:'family',name:'Family',color:'#388d79',favorites:[],watched:[]}];
 const viewers:DemoViewer[]=saved.viewers.filter(v=> 'version' in saved && saved.version===2 || !legacySeeds.some(seed=> v.id===seed.id && v.name===seed.name && v.color===seed.color && !v.photo && JSON.stringify(v.favorites)===JSON.stringify(seed.favorites) && JSON.stringify(v.watched)===JSON.stringify(seed.watched))).map(v=>({...v,photo:typeof v.photo==='string' && v.photo.length<=262144 && /^data:image\/jpeg;base64,[A-Za-z0-9+/]+={0,2}$/.test(v.photo)?v.photo:undefined}));
 if(!viewers.length) return structuredClone(initialDemoState);
 return {version:2,viewers,activeId:'activeId' in saved && typeof saved.activeId==='string' && viewers.some(v=>v.id===saved.activeId)?saved.activeId:null};
 }
 }} catch { /* Keep the prototype usable without storage. */ }
 return structuredClone(initialDemoState);
}
export function persistDemoState(state:DemoState):boolean{try{localStorage.setItem(stateKey,JSON.stringify({...state,version:2}));window.dispatchEvent(new Event('piratarr:viewer-change'));return true}catch{return false}}
