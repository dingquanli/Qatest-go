import{c as a}from"./utils-2MHIbzqG.js";import{I as n}from"./index-BOsbMlOz.js";/**
 * @license lucide-vue-next v1.0.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const y=a("bell",[["path",{d:"M10.268 21a2 2 0 0 0 3.464 0",key:"vwvbt9"}],["path",{d:"M3.262 15.326A1 1 0 0 0 4 17h16a1 1 0 0 0 .74-1.673C19.41 13.956 18 12.499 18 8A6 6 0 0 0 6 8c0 4.499-1.411 5.956-2.738 7.326",key:"11g9vi"}]]);/**
 * @license lucide-vue-next v1.0.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const g=a("server",[["rect",{width:"20",height:"8",x:"2",y:"2",rx:"2",ry:"2",key:"ngkwjq"}],["rect",{width:"20",height:"8",x:"2",y:"14",rx:"2",ry:"2",key:"iecqi9"}],["line",{x1:"6",x2:"6.01",y1:"6",y2:"6",key:"16zg32"}],["line",{x1:"6",x2:"6.01",y1:"18",y2:"18",key:"nzw8ys"}]]);/**
 * @license lucide-vue-next v1.0.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const m=a("wifi",[["path",{d:"M12 20h.01",key:"zekei9"}],["path",{d:"M2 8.82a15 15 0 0 1 20 0",key:"dnpr2z"}],["path",{d:"M5 12.859a10 10 0 0 1 14 0",key:"1x1e6c"}],["path",{d:"M8.5 16.429a5 5 0 0 1 7 0",key:"1bycff"}]]),r=()=>n.get("/settings"),c=e=>n.put("/settings",e),v=e=>n.get(`/settings/${encodeURIComponent(e)}`),h=(e,t)=>n.put(`/settings/${encodeURIComponent(e)}`,{value:t});function o(e="id"){return`${e}-${Date.now()}-${Math.random().toString(36).slice(2,7)}`}function s(){const e="env-default";return{theme:"light",defaultTimeout:3e4,environments:[{id:e,name:"默认环境",baseUrl:"",variables:[{id:o("v"),key:"",value:"",enabled:!0}]}],activeEnvId:e,notifications:{taskFailed:!0,apiError:!0,taskStart:!1},network:{mode:"intranet",intranetUrl:"",extranetUrl:""},bugSync:{jira:{enabled:!1,baseUrl:"",project:"",username:"",apiToken:""},feishu:{enabled:!1,webhookUrl:""},wecom:{enabled:!1,webhookUrl:""}}}}function u(e){return{theme:e.theme,defaultTimeout:String(e.defaultTimeout),activeEnvId:e.activeEnvId,environments:JSON.stringify(e.environments),notifications:JSON.stringify(e.notifications),network:JSON.stringify(e.network),bugSync:JSON.stringify(e.bugSync)}}function d(e){const t=s();if(!e||Object.keys(e).length===0)return t;try{return{theme:["light","dark","system"].includes(e.theme)?e.theme:t.theme,defaultTimeout:e.defaultTimeout&&parseInt(e.defaultTimeout,10)||t.defaultTimeout,environments:e.environments?JSON.parse(e.environments):t.environments,activeEnvId:e.activeEnvId||t.activeEnvId,notifications:e.notifications?JSON.parse(e.notifications):t.notifications,network:e.network?JSON.parse(e.network):t.network,bugSync:e.bugSync?JSON.parse(e.bugSync):t.bugSync}}catch{return t}}function k(){async function e(){const i=await r();return d(i)}async function t(i){await c(u(i))}return{load:e,save:t,defaultSettings:s,uid:o}}export{y as B,g as S,m as W,h as a,s as d,v as g,k as u};
