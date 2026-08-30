import{c as a}from"./utils-Dkx6roOA.js";import{I as t}from"./index-C6R0vIee.js";/**
 * @license lucide-vue-next v1.0.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const c=a("clipboard-check",[["rect",{width:"8",height:"4",x:"8",y:"2",rx:"1",ry:"1",key:"tgr4d6"}],["path",{d:"M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2",key:"116196"}],["path",{d:"m9 14 2 2 4-4",key:"df797q"}]]),p=()=>t.get("/test-plans"),l=e=>t.post("/test-plans",e),d=(e,s)=>t.put(`/test-plans/${encodeURIComponent(e)}`,s),r=e=>t.delete(`/test-plans/${encodeURIComponent(e)}`),u=(e,s={})=>t.post(`/test-plans/${encodeURIComponent(e)}/execute`,s),i=()=>t.get("/plan-executions"),g=()=>t.get("/auto-task-executions");export{c as C,p as a,g as b,l as c,r as d,u as e,i as g,d as u};
