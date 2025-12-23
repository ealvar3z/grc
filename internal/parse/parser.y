%token ANDAND BACKBACK BANG CASE COUNT DUP ELSE END FLAT FN FOR IF IN
%token OROR PIPE REDIR SREDIR SUB SUBSHELL SWITCH TWIDDLE WHILE WORD HUH
/* operator priorities -- lowest first */
%left '^' '='
%right ELSE TWIDDLE
%left WHILE ')'
%left ANDAND OROR '\n'
%left BANG SUBSHELL
%left PIPE
%left PREDIR
%right '$'
%left SUB
%{
package parse

var parseResult *Node

var lastword int
%}
%union {
	node *Node
}
%type<node> rc assign body brace case cbody cmd cmdsa cmdsan comword epilog
%type<node> first line nlwords paren redir sword simple iftail word words
%type<node> arg args else keyword
%type<node> WORD REDIR SREDIR DUP PIPE
%%
rc:	line end		{$$=$1; parseResult=$$; return 1;}
|	error end		{parseResult=nil; return 1;}
end:	END
|	'\n'
cmdsa:	cmd ';'
|	cmd '&'			{$$=N(KBg, $1, nil);}
line:	cmd
|	cmdsa line		{$$=N(KSeq, $1, $2);}
body:	cmd
|	cmdsan body		{$$=N(KSeq, $1, $2);}
cmdsan:	cmdsa
|	cmd '\n'		{$$=$1;}
brace:	'{' body '}'		{$$=N(KBrace, $2, nil);}
paren:	'(' body ')'		{$$=N(KParen, $2, nil);}
assign:	first '=' word		{$$=N(KAssign, $1, $3);}
epilog:				{$$=nil;}
|	redir epilog		{$$=L(KRedir, $1, $2);}
redir:	DUP			{$$=$1;}
|	REDIR word		{$$=$1; $$.Right = $2;}
|	SREDIR word		{$$=$1; if $$.Right == nil { $$.Right = $2; }}
case:	CASE words ';'		{$$=N(KCase, $2, nil);}
|	CASE words '\n'		{$$=N(KCase, $2, nil);}
cbody:	cmd			{$$=N(KCbody, $1, nil);}
|	case cbody		{$$=N(KCbody, $1, $2);}
|	cmdsan cbody		{$$=N(KCbody, $1, $2);}
iftail:	cmd else		{ if $2 != nil { $$=N(KElse, $1, $2); } else { $$=$1; } }
else:				%prec ELSE	{$$=nil;}
|	ELSE optnl cmd		{$$=$3;}
cmd:				%prec WHILE	{$$=nil;}
|	simple			{$$=buildCallFromSimple($1);}
|	brace epilog		{$$=N(KBrace, $1, $2);}
|	IF paren optnl iftail	{$$=N(KIf, $2, $4);}
|	FOR '(' word IN words ')' optnl cmd
				{n:=N(KFor, $3, $8); if $5 != nil { n.List = $5.List; }; $$=n;}
|	FOR '(' word ')' optnl cmd
				{$$=N(KFor, $3, $6);}
|	WHILE paren optnl cmd	{$$=N(KWhile, $2, $4);}
|	SWITCH '(' word ')' optnl '{' cbody '}'
				{$$=N(KSwitch, $3, $7);}
|	TWIDDLE optcaret word words	{$$=N(KMatch, $3, $4);}
|	cmd ANDAND optnl cmd	{$$=N(KAnd, $1, $4);}
|	cmd OROR optnl cmd	{$$=N(KOr, $1, $4);}
|	cmd PIPE optnl cmd	{$$=N(KPipe, $1, $4); if $2 != nil { $$.I1=$2.I1; $$.I2=$2.I2; }}
|	redir cmd  %prec PREDIR	{$$=N(KPre, $1, $2);}
|	assign cmd %prec BANG	{$$=N(KPre, $1, $2);}
|	BANG optcaret cmd	{$$=N(KBang, $3, nil);}
|	SUBSHELL optcaret cmd	{$$=N(KSubshell, $3, nil);}
|	FN words brace		{$$=N(KFnDef, $2, $3);}
|	FN words  %prec ELSE	{$$=N(KFnRm, $2, nil);}
optcaret:
|	'^'
simple:	first		%prec ELSE
|	first args	%prec ELSE	{$$=L(KArgList, $1, $2);}
args:	arg
|	args arg		{$$=L(KArgList, $1, $2);}
arg:	word
|	redir
first:	comword
|	first '^' sword		{$$=N(KConcat, $1, $3);}
sword:	comword
|	keyword			{$$=$1;}
word:	sword
|	word '^' sword		{$$=N(KConcat, $1, $3);}
comword: '$' sword		{$$=N(KVar, $2, nil);}
|	'$' sword SUB words ')'	{$$=N(KVar, $2, $4);}
|	COUNT sword		{$$=N(KCount, $2, nil);}
|	FLAT sword		{$$=N(KFlat, $2, nil);}
|	'`' sword		{$$=N(KBackquote, nil, $2);}
|	'`' brace		{$$=N(KBackquote, nil, $2);}
|	BACKBACK word brace	{$$=N(KBackquote, $2, $3);}
|	BACKBACK word sword	{$$=N(KBackquote, $2, $3);}
|	'(' nlwords ')'		{$$=N(KParen, $2, nil);}
|	REDIR brace		{$$=N(KNmpipe, $1, $2);}
|	WORD
keyword: FOR		{$$=W("for");}
|	IN		{$$=W("in");}
|	WHILE		{$$=W("while");}
|	IF		{$$=W("if");}
|	SWITCH		{$$=W("switch");}
|	FN		{$$=W("fn");}
|	CASE		{$$=W("case");}
|	TWIDDLE		{$$=W("~");}
|	BANG		{$$=W("!");}
|	SUBSHELL	{$$=W("@");}
|	'='		{$$=W("=");}
words:				{$$=nil;}
|	words word		{$$=L(KWords, $1, $2);}
nlwords:			{$$=nil;}
|	nlwords '\n'
|	nlwords word		{$$=L(KWords, $1, $2);}
optnl:
|	optnl '\n'
%%
