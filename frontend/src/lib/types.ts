export type Scenario = {id:string;name:string;description:string;expected:string};
export type Event = {at:string;kind:string;message:string;hash?:string;status?:number};
export type Evidence = {hash:string;confirmed:boolean;successful:boolean;ledger?:number;explorer:string;amount?:string;verification_error?:string};
export type Client = {id:string;status:string;source?:string;destination?:string;session_id?:string;attempts:number;settlements:number;received:string;verdict:string;events:Event[];transactions:Evidence[]};
export type Run = {id:string;scenario:string;status:string;created_at:string;finished_at?:string;network:string;asset:string;amount:string;error?:string;clients:Client[]};
export type Session = {id:string;scenario:string;attempts:number;events:Event[];transactions:Evidence[]};
export type Challenge = {id:string;status:string;created_at:string;finished_at?:string;deadline?:string;choice?:string;scenario?:string;error?:string;recipient:string;amount:string;result:Client};
