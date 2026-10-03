<script lang="ts">
 import {onMount,tick} from 'svelte';
 import type {Challenge} from '$lib/types';
 let {request,busy,locked=false}:{request:(path:string,init?:RequestInit)=>Promise<any>;busy:boolean;locked?:boolean}=$props();
 let payout=$state<Challenge|null>(null),key=$state(''),starting=$state(false),restoring=$state(false),choosing=$state(false),error=$state(''),now=$state(Date.now());
 let decisionHeading=$state<HTMLHeadingElement|null>(null);
 let previousPhase='';
 $effect(()=>{const phase=payout?.status;if(phase==='waiting'&&previousPhase!==phase&&key){tick().then(()=>{if(payout?.status==='waiting'){decisionHeading?.focus({preventScroll:true});if(window.innerWidth<=700)decisionHeading?.scrollIntoView({block:'start',behavior:'instant'});}});}previousPhase=phase||'';});
 let active=$derived(!!payout&&['funding','sending','waiting','recovering','verifying'].includes(payout.status));
 let terminal=$derived(!!payout&&!active);
 let verified=$derived(payout?.result.status==='complete');
 let seconds=$derived(payout?.deadline?Math.max(0,Math.ceil((new Date(payout.deadline).getTime()-now)/1000)):0);
 let faultNames:Record<string,string>={before_submit:'Stopped before broadcast',lost_confirmation:'Settled, but the reply was lost',delayed_lookup:'Settled, with confirmation temporarily hidden'};
 const number=(n:number|string)=>new Intl.NumberFormat(undefined,{maximumFractionDigits:7}).format(Number(n));
 const short=(hash:string)=>`${hash.slice(0,10)}…${hash.slice(-8)}`;
 const clock=(n:number)=>`${Math.floor(n/60)}:${String(n%60).padStart(2,'0')}`;
 async function refresh(){if(!payout)return;const id=payout.id;try{const data=await request(`/api/challenges/${id}`);if(payout?.id===id)payout=data;}catch(e){error=(e as Error).message;}}
 onMount(()=>{
  const id=new URL(window.location.href).searchParams.get('challenge')||sessionStorage.getItem('ledgerdrill-last-challenge');
  if(id){restoring=true;key=sessionStorage.getItem(`ledgerdrill-choice-${id}`)||'';request(`/api/challenges/${id}`).then(data=>{payout=data;}).catch(e=>{error=e.message;}).finally(()=>restoring=false);}
  const poll=setInterval(()=>{now=Date.now();if(active&&document.visibilityState==='visible')refresh();},1500);
  return()=>clearInterval(poll);
 });
 async function start(){starting=true;error='';try{
  const data=await request('/api/challenges',{method:'POST',body:'{}'});payout=data.challenge;key=data.decision_key;
  sessionStorage.setItem(`ledgerdrill-choice-${payout!.id}`,key);sessionStorage.setItem('ledgerdrill-last-challenge',payout!.id);
  const url=new URL(window.location.href);url.search='';url.searchParams.set('view','challenge');url.searchParams.set('challenge',payout!.id);window.history.replaceState({},'',url);
 }catch(e){error=(e as Error).message;}finally{starting=false;}}
 async function choose(action:string){if(!payout)return;choosing=true;error='';try{
  await request(`/api/challenges/${payout.id}/decision`,{method:'POST',body:JSON.stringify({key,action})});payout={...payout,status:'recovering',choice:action};
 }catch(e){error=(e as Error).message;await refresh();}finally{choosing=false;}}
 async function download(){if(!payout)return;try{
  const report=await request(`/api/challenges/${payout.id}/report`);const url=URL.createObjectURL(new Blob([JSON.stringify(report,null,2)],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download=`ledgerdrill-payout-${payout.id}.json`;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
 }catch(e){error=(e as Error).message;}}
</script>

<div class="intro challenge-intro"><div><h1>Don’t pay Rowan twice.</h1><p>One payout. A broken connection. You decide what happens next.</p></div><p class="challenge-footnote">Fictional recipient.<br/>Real Stellar testnet payments.</p></div>
{#if error}<div class="error-message" role="alert">{error}<button class="text-button" onclick={()=>error=''}>Dismiss</button></div>{/if}
<div class="payout-layout">
 <section class="payout-receipt" aria-label="Your payout request">
  <div class="receipt-person"><svg width="38" height="38" viewBox="0 0 38 38" aria-hidden="true"><circle cx="19" cy="19" r="18" fill="none" stroke="currentColor"/><path d="M12 28v-2a7 7 0 0 1 14 0v2M23 14a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z" fill="none" stroke="currentColor" stroke-width="1.5"/></svg><div><span>Pay Rowan</span><p>One completed job. One payment owed.</p></div></div>
  <div class="payout-amount"><span class="mono">1</span><span>XLM</span></div>
  <dl class="payout-fields"><div><dt>Payment intent</dt><dd>Pay once</dd></div><div><dt>Network</dt><dd>Stellar testnet</dd></div><div><dt>Confirmation</dt><dd class:uncertain={payout?.status==='waiting'}>{!payout?'Not sent':payout.status==='funding'?'Setting up':payout.status==='waiting'?'Unknown':terminal&&verified?'Ledger checked':'Checking…'}</dd></div></dl>
  {#if !payout}<button class="button primary" disabled={starting||restoring||busy||locked} onclick={start}>{starting?'Starting payout…':busy?'Lab is in use…':'Send 1 XLM'}<svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 12h15m-5-5 5 5-5 5" fill="none" stroke="currentColor" stroke-width="1.8"/></svg></button><p class="receipt-note">{#if busy}Another payout is using the shared lab. <a href="?view=history">See the saved payouts</a>.{:else}No wallet needed. We create fresh test accounts and deliberately interrupt the response.{/if}</p>{:else}<p class="receipt-note">Payout <span class="mono" translate="no">{payout.id.slice(0,8)}</span><br/>Your intent stays at 1 XLM even if recovery sends more.</p>{/if}
 </section>
 <section class="payout-action" aria-live="polite" aria-atomic="false">
 {#if !payout}
  <div class="challenge-empty"><h2>A timeout doesn’t tell you who got paid.</h2><p>Start the payout. Somewhere between your app and Stellar, the confirmation will go missing.</p><p>You’ll have a choice: send another payment, or find out what happened to the first one.</p><div class="challenge-rule"><span class="dot"></span><p>The outcome comes from actual ledger transactions. Your choice controls the recovery.</p></div></div>
 {:else if payout.status==='funding'||payout.status==='sending'}
  <div class="challenge-working" role="status"><span class="signal" aria-hidden="true"></span><h2>{payout.status==='funding'?'Opening fresh test accounts…':'Sending your payout…'}</h2><p>{payout.status==='funding'?'Friendbot is funding a sender and recipient. This usually takes a few ledger closes.':'The payment is going through a controlled connection failure. We haven’t chosen a recovery for you.'}</p><p class="small muted">Keep this tab open. No real funds are involved.</p></div>
 {:else if payout.status==='waiting'}
  <div class="decision-heading"><svg width="40" height="40" viewBox="0 0 40 40" aria-hidden="true"><path d="M5 20h10m10 0h10M17 13l6 14m0-14-6 14" fill="none" stroke="currentColor" stroke-width="2"/></svg><h2 bind:this={decisionHeading} tabindex="-1">No confirmation came back.</h2></div>
  <p class="decision-copy">Rowan might have been paid. Or the payment might never have reached Stellar. Your app has an error, not an answer.</p>
  {#if key}<h3>What do you do?</h3><div class="recovery-choices"><button disabled={choosing||seconds===0} onclick={()=>choose('retry')}><span>Send another 1 XLM</span><p>Create a fresh payment and try again.</p><svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 12h15m-5-5 5 5-5 5" fill="none" stroke="currentColor" stroke-width="1.6"/></svg></button><button disabled={choosing||seconds===0} onclick={()=>choose('reconcile')}><span>Find the original payment</span><p>Check its hash. Reuse its signed envelope if needed.</p><svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true"><circle cx="10" cy="10" r="6" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="m15 15 5 5" stroke="currentColor" stroke-width="1.6"/></svg></button></div><p class="decision-time">Choose within <span class="mono" aria-hidden="true">{clock(seconds)}</span><span class="sr-only">2 minutes</span>. Without a choice, no additional payment will be sent.</p>{:else}<p class="decision-time">The browser that started this payout has the recovery controls. You can inspect the result after it finishes.</p>{/if}
 {:else if payout.status==='recovering'||payout.status==='verifying'}
  <div class="challenge-working" role="status"><span class="signal" aria-hidden="true"></span><h2>{payout.status==='verifying'?'Opening the ledger…':payout.choice==='retry'?'Sending a new payment…':'Finding your original payment…'}</h2><p>{payout.status==='verifying'?'We’re checking every submitted hash directly against Stellar, outside the fault proxy.':'Your choice is being executed on testnet. The result will show what actually settled.'}</p></div>
 {:else}
  <div class="payout-reveal" class:duplicate={verified&&payout.result.settlements>1} class:risky={payout.choice==='retry'} class:unresolved={payout.status!=='complete'}>
   <h2>{payout.status==='expired'?'No recovery was chosen.':payout.status!=='complete'?'The result is unresolved.':payout.result.settlements>1?'Rowan got paid twice.':payout.choice==='retry'?'One payment, this time.':'One payout. No duplicate.'}</h2>
   {#if payout.status==='complete'}<p>{payout.result.settlements>1?'The first payment had already settled. Your fresh retry created a second transaction—and a second payout.':payout.choice==='retry'?'Only the fresh payment settled this time. The same retry can pay twice when the original reaches the network.':'You kept the original payment intent. Checking its hash and retaining its envelope recovered the payout without creating a second payment.'}</p>{:else}<p>{payout.error}</p>{/if}
   <dl class="revealed-facts"><div><dt>You meant to send</dt><dd class="mono">1 <small>XLM</small></dd></div><div><dt>Verified received</dt><dd class="mono">{verified?number(payout.result.received):'Unknown'}{#if verified}<small> XLM</small>{/if}</dd></div><div><dt>Unique payments</dt><dd class="mono">{verified?number(payout.result.settlements):'Unknown'}</dd></div></dl>
   {#if payout.scenario}<p class="fault-reveal">The failure: <strong>{faultNames[payout.scenario]||payout.scenario}</strong></p>{/if}
   {#if payout.result.transactions.length}<h3>See the payments yourself</h3><ul class="payout-proof">{#each payout.result.transactions as tx}<li><a class="mono" href={tx.explorer} target="_blank" rel="noreferrer" translate="no">{short(tx.hash)} <svg width="12" height="12" viewBox="0 0 16 16" aria-hidden="true"><path d="M4 12 12 4M5 4h7v7" fill="none" stroke="currentColor" stroke-width="1.4"/></svg></a><span>{tx.verification_error?'Check unavailable':tx.confirmed&&tx.successful?`${number(tx.amount||'0')} XLM · ledger ${tx.ledger}`:tx.confirmed?'Included but failed':'Not found during ledger check'}</span></li>{/each}</ul>{/if}
   <div class="payout-next"><button class="button primary" disabled={starting||restoring||busy||locked} onclick={start}>{starting?'Starting…':'Try another failure'}</button><button class="text-button" onclick={download}>Export your evidence</button><a class="text-button" href={`?view=lab&scenario=${payout.scenario||'lost_confirmation'}`}>Compare both strategies</a></div>
  </div>
 {/if}
 </section>
</div>
{#if terminal&&payout?.result.events.length}<details class="challenge-trace"><summary>What happened on the connection</summary><ol class="timeline">{#each payout.result.events as event}<li><span class="mono event-time">{event.kind}</span><div><p>{event.message}</p>{#if event.hash}<span class="mono hash" translate="no">{event.hash}</span>{/if}</div></li>{/each}</ol></details>{/if}
