# Race Conditions

Bajo las siguientes condiciones se podrían haber presentado situaciones de race conditions:

## Deposit

El caso de uso _Deposit_ del dominio de _Payments_ no tiene problemas con race conditions.
En principio, podrían llegar multiples solicitudes simultáneas (incluso del mismo cliente) y podrían procesarse los cobros al provider de forma individual y actualizar las bases de datos sin interferirse.

## Purchase

*Double Debit*
Puede darse la situación de que lleguen al menos dos peticiones, que validen que un usuario está activo y no tiene asignado un servicio. Luego, proceden a debitar el monto del servicio guardando los datos de la transacción y, finalmente, asignar el acceso al servicio. Entonces, mientras hubiera balance disponible, se podría debitar dos veces el monto del servicio para luego una de las peticiones (en caso de ser dos) encontrarse con que ya se había procesado la asignación del servicio y retornar eso (sin gestionar el monto erroneamente debitado).

## Refund

*Double Credit*
El flujo parte similar al de _Purchase_ donde primero valida el usuario y que el servicio fuera asignado al usuario, puntos de validación que si hubiera más de una solicitud serían confirmadas. Luego, se procedería a creditar, proceso que haría cada una de las solicitudes concurrentes sin fallar. Luego, se intentaría el quitado de los permisos a los servicios, proceso que sólo acertaría una vez y luego retornaría error (pero sin debitar el monto erroneamente acreditado).

# Apendix

Estas situaciones fueron resueltas con la implementación de transacciones que ejecutan las operaciones de forma atómica, es decir, si una operación falla, todas las operaciones anteriores también fallarán y se deshacerán. Esto aplica particularmente en las operaciones de _Debit_ y Credit_ cuando se hace manejo de los _Entitlements_ (dando o quitando accesos).