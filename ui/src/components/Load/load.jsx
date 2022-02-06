import React, {useEffect, useState} from "react";
import Form from "./Form/Form";
import Info from "./Info/Info";
import Bar from "./Bar/Bar";
import Client from "../../api/client"
import RefreshIcon from '@mui/icons-material/Refresh';
import {Button, Collapse, LinearProgress} from "@mui/material";


const Load = () => {
    const [data, setData] = useState({});
    const [formData, setFormData] = useState({})

    const [lock, setLock] = useState(true)

    const [loading, setLoading] = useState(false)

    useEffect(() => {
        refresh()
    }, [formData])
    
    const refresh = () => {

        let params = {
            "coin": formData.coin,
            "from": formData.from,
            "to": formData.to,
        }

        console.log(formData)

        setLoading(true)

        Client("history").call(params, (data) => {
            let points = data.map((p, _) => {
                return {
                    x: new Date(p.From),
                    y: 1,
                }
            })
            setData({
                coin: formData.coin,
                time: points,
            })
            setLoading(false)
        })
    }

    const handleChange = (data) => {
        setFormData(data)
    }

    const submitChange = (data) => {
        console.log(data)
        Client("load").call({
            "coin": formData.coin,
            "from": formData.from,
            "to": formData.to,
        }, (data) => {
            console.log(data)
        })
    }

    // const handleDelete = id => {
    //   storeData('lastState', state);
    //   let newState = state.filter(i => {
    //     return i.id !== id;
    //   });
    //   setState(newState);
    // };

    return (
        <div className='container'>
            <Collapse in={loading}>
                <LinearProgress/>
            </Collapse>
            <div className='row'>
                <div className='col m12 s12'>
                    <Button onClick={refresh}>
                        <RefreshIcon/>
                    </Button>
                    <Form change={handleChange} submit={submitChange}/>
                    <Bar coin={data.coin}
                         labelData={data.time}
                    />
                    <div>
                        <div className='row center'>
                            <h4 className='white-text'>7 Day Data</h4>
                        </div>
                        <div className='data-container row'>
                            {data.details && data.details.length > 0 ? (
                                <>
                                    {data.details.map(detail => (
                                        <Info
                                            coin={detail.coin}
                                            duration={detail.duration}
                                            fees={detail.result.fees}
                                            trades={detail.result.trades}
                                            pnl={detail.result.pnl}
                                            value={detail.result.value}
                                            coins={detail.result.coins}
                                            threshold={detail.result.threshold}
                                            prev={detail.prev}
                                            next={detail.next}
                                            // deleteCard={handleDelete}
                                        />
                                    ))}
                                </>
                            ) : (
                                <div className='center white-text'>No log found</div>
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Load;